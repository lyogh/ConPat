package workerpool

import (
	"context"
	"fmt"
	"log"
	"sync"
)

const (
	LogModeTrace logMode = iota // Все сообщения
	LogModeErr                  // Только ошибки
)

type logMode int

// Пул
type WorkerPool struct {
	wg sync.WaitGroup

	queue  chan *workTask // Очередь задач на обработку
	result chan *workTask // Выполненные задачи

	workers struct {
		max int           // Максимальное количество параллельных горутин
		ch  chan struct{} // Канал горутин. Вместимость канала = workers.max
	}

	logger struct {
		mode logMode // Режим журнала (LogModeTrace, LogModeErr)
	}
}

func NewPool(max int) *WorkerPool {
	w := &WorkerPool{}

	w.workers.max = max
	w.workers.ch = make(chan struct{}, w.workers.max)

	w.queue = make(chan *workTask)
	w.result = make(chan *workTask)

	// Сразу планируем получение результата обработки задач
	go w.receive()

	return w
}

// Получает результат обработки задач от параллельных обработчиков
func (p *WorkerPool) receive() {
	for r := range p.result {
		p.log(LogModeTrace, fmt.Sprintf("результат обработки получен %v", r))
	}
}

// Отправляет задачу на выполнение
func (p *WorkerPool) Do(ctx context.Context, t *Task, hndl ...Handler) {
	for _, h := range hndl {
		p.createWorker(ctx)

		select {
		// Отправляем задачу на обработку в канал
		case p.queue <- &workTask{
			Task:    t,
			Handler: h,
		}:
			p.log(LogModeTrace, fmt.Sprintf("задача %v успешно отправлена на обработку", t))
			// Отмена выполнения
		case <-ctx.Done():
			p.log(LogModeTrace, fmt.Sprintf("отмена задачи %v", t))
		}
	}
}

// Планирует горутину для обработки задачи
func (p *WorkerPool) createWorker(ctx context.Context) {
	p.wg.Add(1)

	// Пытаемся отправить что-нибудь в канал.
	// Если превышен размер буфера канала, тогда ждем его освобождения (см. ниже "<-p.workers.ch")
	p.workers.ch <- struct{}{}

	// Планируем горутину
	go func(ctx context.Context) {
		defer func() {
			p.wg.Done()
			// Освобождаем место в канале для следующего обработчика ( см. выше "p.workers.ch <- struct{}{}")
			<-p.workers.ch
		}()

		p.log(LogModeTrace, "обработчик запущен")

		select {
		// Получаем задачу на обработку
		case ht := <-p.queue:
			r, err := ht.Do(ht.Task)
			ht.addResult(&TaskResult{data: r, err: err})
			select {
			// Отправляем результат
			case p.result <- ht:
				p.log(LogModeTrace, fmt.Sprintf("результат обработки задачи %v отправлен", ht.Task))
				// или прерываем отправку при отмене
			case <-ctx.Done():
				p.log(LogModeTrace, fmt.Sprintf("отмена обработчика задачи %v (без отправки результата)", ht.Task))
				return
			}
		case <-ctx.Done():
			p.log(LogModeTrace, "отмена обработчика (без получения задачи)")
			return
		}
	}(ctx)

	p.log(LogModeTrace, fmt.Sprintf("количество параллельных обработчиков = %d", len(p.workers.ch)))
}

// Ожидает обработку всех задач
func (p *WorkerPool) Wait() {
	p.wg.Wait()
}

// Добавляет сообщение в журнал
func (p *WorkerPool) log(m logMode, v any) {
	if m == p.logger.mode || m == LogModeTrace {
		log.Println(v)
	}
}

// Закрывает пул
func (p *WorkerPool) Close() {
	close(p.queue)
	close(p.result)
	close(p.workers.ch)
}
