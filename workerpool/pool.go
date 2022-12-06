package workerpool

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
)

const (
	LogModeNone  logMode = iota // Без журналирования
	LogModeTrace                // Все сообщения
	LogModeErr                  // Только ошибки
)

type logMode int

func (q queue) String() string {
	var b strings.Builder

	for _, t := range q {
		b.WriteString(fmt.Sprintf("[%v]", t))
	}

	return b.String()
}

// Пул
type WorkerPool struct {
	wg sync.WaitGroup
	sync.RWMutex

	queue

	tasks  chan *workTask // Очередь задач на обработку
	result chan *workTask // Выполненные задачи

	workers struct {
		max int           // Максимальное количество параллельных горутин
		ch  chan struct{} // Канал горутин. Вместимость канала = workers.max
	}

	logger struct {
		mode logMode // Режим журнала (LogModeTrace, LogModeErr)
	}
}

// Создает новый пул
func NewPool(max int) *WorkerPool {
	w := &WorkerPool{}

	w.workers.max = max
	w.workers.ch = make(chan struct{}, w.workers.max)

	w.tasks = make(chan *workTask)
	w.result = make(chan *workTask)

	// Сразу планируем получение результата обработки задач
	go w.receive()

	return w
}

// Режим журналирования сообщений
func (p *WorkerPool) SetLogMode(m logMode) {
	p.logger.mode = m
}

// Получает результат обработки задач от параллельных обработчиков
func (p *WorkerPool) receive() {
	for r := range p.result {
		p.wg.Done()
		p.log(LogModeTrace, r.Task, fmt.Sprintf("результат обработки %v получен", r.Handler))
	}
}

// Отправляет задачу на выполнение
func (p *WorkerPool) Do(t *Task, hndl ...Handler) {
	for _, h := range hndl {
		p.createWorker()

		wt := &workTask{
			Task:    t,
			Handler: h,
		}

		p.wg.Add(1)

		select {
		// Отправляем задачу на обработку в канал
		case p.tasks <- wt:
			p.log(LogModeTrace, t, fmt.Sprintf("успешно отправлена на обработку %v", wt.Handler))
			// Сразу не получилось, отправим позже, на вызове метода Wait
		default:
			/*go func() {
				p.tasks <- wt
			}()
			p.log(LogModeTrace, t, fmt.Sprintf("нет свободных обработчиков, обработка %v будет выполнена позже", wt.Handler))*/
			p.addToQueue(wt)
		}
	}
}

// Добавляет задачу в очередь
func (p *WorkerPool) addToQueue(wt *workTask) {
	defer p.Unlock()

	p.Lock()

	p.queue = append(p.queue, wt)

	p.log(LogModeTrace, wt.Task, fmt.Sprintf("добавлена в очередь. Очередь: %v", p.queue))
}

// Возвращает задачу из очереди
func (p *WorkerPool) getFromQueue() *workTask {
	defer p.Unlock()

	p.Lock()

	if len(p.queue) == 0 {
		return nil
	}

	wt := p.queue[0]
	p.queue = p.queue[1:]

	return wt
}

// Отправляет задачи накопившиеся в очереди на обработку
func (p *WorkerPool) flush() {
	for {
		wt := p.getFromQueue()
		if wt != nil {
			p.createWorker()

			select {
			// Отправляем задачу на обработку в канал
			case p.tasks <- wt:
				p.log(LogModeTrace, wt.Task, fmt.Sprintf("успешно отправлена из очереди. Очередь: %v", p.queue))
				// или возвращаем в очередь
			default:
				p.addToQueue(wt)
			}
		}

		runtime.Gosched()
	}
}

// Планирует горутину для обработки задачи
func (p *WorkerPool) createWorker() {
	// Пытаемся отправить что-нибудь в канал.
	// Если превышен размер буфера канала, тогда выходим
	select {
	case p.workers.ch <- struct{}{}:
	default:
		return
	}

	// Планируем горутину
	go func() {
		defer func() {
			// Освобождаем место в канале для следующего обработчика ( см. выше "p.workers.ch <- struct{}{}")
			<-p.workers.ch
		}()

		// Получаем задачу на обработку
		for ht := range p.tasks {
			r, err := ht.Do(ht.Task)
			ht.addResult(&TaskResult{Data: r, Err: err})

			// Отправляем результат
			p.result <- ht
			p.log(LogModeTrace, ht.Task, fmt.Sprintf("результат обработки %v отправлен", ht.Handler))
		}
	}()

	p.log(LogModeTrace, nil, fmt.Sprintf("количество параллельных обработчиков: %d", len(p.workers.ch)))
}

// Ожидает обработку всех задач
func (p *WorkerPool) Wait() {
	go p.flush()

	p.log(LogModeTrace, nil, "ожидание завершения обработки")
	p.wg.Wait()
}

// Добавляет сообщение в журнал
func (p *WorkerPool) log(m logMode, t *Task, v any) {
	var prefix string

	if p.logger.mode == LogModeNone ||
		m != p.logger.mode && m != LogModeTrace {
		return
	}

	if t != nil {
		prefix = fmt.Sprintf("%v: ", t)
	}

	log.Printf("%s%v\n", prefix, v)
}

// Закрывает пул
func (p *WorkerPool) Close() {
	close(p.tasks)
	close(p.result)
	close(p.workers.ch)
}
