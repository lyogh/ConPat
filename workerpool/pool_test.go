package workerpool

import (
	"fmt"
	"log"
	"reflect"
	"runtime"
	"testing"
)

// Добавляет к данным скобки
type Formatter struct {
}

func (p *Formatter) Do(t *Task) (any, error) {
	return fmt.Sprintf("(%v)", t.Data), nil
}

// Выводит в журнал, но ничего не возвращает в результате обработки
type Printer struct {
}

func (p *Printer) Do(t *Task) (any, error) {
	log.Println(t)
	return nil, nil
}

// Умножает значение на 3
type X3Multiplier struct {
}

func (p *X3Multiplier) Do(t *Task) (any, error) {
	return 3 * t.Data.(int), nil
}

type testData struct {
	task    *Task
	handler Handler
	result  *TaskResult
}

func TestWorkerPool(t *testing.T) {
	var test []testData = []testData{
		{task: NewTask("string"), handler: &Printer{}, result: &TaskResult{data: nil, err: nil}},
		{task: NewTask(1), handler: &X3Multiplier{}, result: &TaskResult{data: 3, err: nil}},
		{task: NewTask("string"), handler: &Formatter{}, result: &TaskResult{data: "(string)", err: nil}},
	}

	w := NewPool(runtime.NumCPU())

	for _, td := range test {
		w.Do(td.task, td.handler)
	}

	w.Wait()
	w.Close()

	for _, td := range test {
		res := td.task.Results()
		if len(res) != 1 {
			t.Error("expected", 1, "got", len(res))
		}

		_, ok := res[td.handler]
		if !ok {
			t.Error("expected", true, "got", ok)
		}

		if !reflect.DeepEqual(res[td.handler], td.result) {
			t.Error("expected", res, "got", td.result)
		}
	}

}
