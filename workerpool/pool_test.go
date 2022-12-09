package workerpool

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"runtime"
	"testing"
	"time"
)

// Добавляет к данным скобки
type Formatter struct {
}

func (p *Formatter) Do(cxt context.Context, t *Task) (any, error) {
	return fmt.Sprintf("(%v)", t.Data), nil
}

// Выводит в журнал, но ничего не возвращает в результате обработки
type Printer struct {
}

func (p *Printer) Do(cxt context.Context, t *Task) (any, error) {
	log.Println(t)
	return nil, nil
}

// Умножает значение на 3
type X3Multiplier struct {
}

func (p *X3Multiplier) Do(ctx context.Context, t *Task) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Second * 10):
	}

	return 3 * t.Data.(int), nil
}

type testData struct {
	task    *Task
	handler Handler
	result  *TaskResult
}

func TestWorkerPool(t *testing.T) {
	var test []testData = []testData{
		{task: NewTask("string"), handler: &Printer{}, result: &TaskResult{Data: nil, Err: nil}},
		{task: NewTask(1), handler: &X3Multiplier{}, result: &TaskResult{Data: 3, Err: nil}},
		{task: NewTask("string"), handler: &Formatter{}, result: &TaskResult{Data: "(string)", Err: nil}},
	}

	w, err := NewPool(runtime.NumCPU())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	for _, td := range test {
		w.Do(ctx, td.task, td.handler)
	}

	w.Wait()

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
