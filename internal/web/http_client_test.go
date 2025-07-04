package web

import (
	"context"
	"testing"

	"plexobject.com/formicary/internal/types"
)

func newTest() HTTPClient {
	c := types.CommonConfig{}
	return New(&c)
}

func Test_ShouldRealGet(t *testing.T) {
	w := newTest()
	_, _, err := w.Get(
		context.Background(),
		"https://jsonplaceholder.typicode.com/todos/1",
		map[string]string{"key": "value"},
		make(map[string]string))
	if err != nil {
		t.Logf("Unexpected response Get error %s", err)
	}
}

func Test_ShouldRealDelete(t *testing.T) {
	w := newTest()
	_, _, err := w.Delete(
		context.Background(),
		"https://jsonplaceholder.typicode.com/todos/1",
		map[string]string{"key": "value"},
		nil)
	if err != nil {
		t.Logf("Unexpected response Get error %s", err)
	}
}

func Test_ShouldRealDeleteBody(t *testing.T) {
	w := newTest()
	_, _, err := w.Delete(
		context.Background(),
		"https://jsonplaceholder.typicode.com/todos/1",
		map[string]string{"key": "value"},
		[]byte("hello"))
	if err != nil {
		t.Logf("Unexpected response Get error %s", err)
	}
}

func Test_ShouldRealPostError(t *testing.T) {
	w := newTest()
	_, _, err := w.PostJSON(
		context.Background(),
		"https://jsonplaceholder.typicode.com/todos_____",
		map[string]string{"key": "value"},
		map[string]string{},
		nil)
	if err == nil {
		t.Logf("Expected response PostJSON error ")
	}
}

func Test_ShouldRealPostJSON(t *testing.T) {
	w := newTest()
	_, _, _ = w.PostJSON(
		context.Background(),
		"https://jsonplaceholder.typicode.com/todos",
		map[string]string{"key": "value"},
		map[string]string{},
		nil)
}

func Test_ShouldRealPostForm(t *testing.T) {
	w := newTest()
	_, _, _ = w.PostForm(
		context.Background(),
		"https://jsonplaceholder.typicode.com/todos",
		map[string]string{"header": "value"},
		map[string]string{"name": "value"})
}

func Test_ShouldRealPostBody(t *testing.T) {
	w := newTest()
	_, _, err := w.PostJSON(
		context.Background(),
		"https://jsonplaceholder.typicode.com/todos",
		map[string]string{"key": "value"},
		map[string]string{},
		[]byte("hello"))
	if err != nil {
		t.Logf("Unexpected response PostJSON error %s", err)
	}
}
