package infraoperator

import "testing"

func Test_collection_empty(t *testing.T) {
	var c collection
	if len(c) != 0 {
		t.Error("expected len(c) == 0")
	}
}

func Test_collection_add_one(t *testing.T) {
	var c collection
	c = c.add(1)
	if len(c) != 1 {
		t.Error("expected len(c) == 1")
	}
}

func Test_collection_duplicate(t *testing.T) {
	var c collection
	c = c.add(1)
	c = c.add(1)
	if len(c) != 1 {
		t.Error("expected len(c) == 1")
	}
}

func Test_collection_remove(t *testing.T) {
	var c collection
	c = c.add(1)
	c = c.remove(1)

	if len(c) != 0 {
		t.Error("expected len(c) == 0")
	}
}

func Test_collection_remove_invalid(t *testing.T) {
	var c collection
	c = c.add(1)
	c = c.remove("not exists")

	if len(c) != 1 {
		t.Error("expected len(c) == 1")
	}
}

func Test_collection_ptr(t *testing.T) {
	var c collection

	var (
		a = 1
		b = 2
	)

	c = c.add(&a)
	c = c.add(&b)

	c = c.remove(a) // should not remove
	if len(c) != 2 {
		t.Error("expected len(c) == 2")
	}

	c = c.remove(&a) // should remove
	if len(c) != 1 {
		t.Error("expected len(c) == 1")
	}
}
