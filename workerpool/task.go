package workerpool

import (
	"context"
	"fmt"
	"sync"
)

// Результат обработки задачи
type TaskResult struct {
	Data any   // Данные
	Err  error // или ошибка
}

// Задача на обработку
type Task struct {
	Data   any                     // Данные задачи
	result map[Handler]*TaskResult // Результат выполнения задачи каждым обработчиком
}

// Задача в работе
type workTask struct {
	sync.Mutex                 // Семафор
	*Task                      // Задача
	Handler                    // Обработчик
	ctx        context.Context // Контекст
}

// Обработчик задачи
type Handler interface {
	Do(context.Context, *Task) (any, error)
}

type queue []*workTask

// Создает задачу
func NewTask(d any) *Task {
	t := &Task{Data: d}

	t.result = make(map[Handler]*TaskResult)

	return t
}

// Возвращает результаты обработки задач всеми обработчиками
func (t *Task) Results() map[Handler]*TaskResult {
	return t.result
}

func (t *Task) String() string {
	return fmt.Sprintf("Задача [%v]", t.Data)
}

// Возвращает первый результат обработки задач
func (t *Task) Result() *TaskResult {
	for _, r := range t.result {
		return r
	}

	return nil
}

// Сохраняет результат обработки задачи
func (wt *workTask) addResult(r *TaskResult) {
	wt.Lock()
	defer wt.Unlock()

	wt.result[wt.Handler] = r
}

func (r *TaskResult) String() string {
	return fmt.Sprintf("Результат обработки [%v,%v]", r.Data, r.Err)
}
