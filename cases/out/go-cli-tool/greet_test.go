package main

import "testing"

func TestGreet(t *testing.T) {
	if got := Greet("Ada"); got != "Hello, Ada!" {
		t.Errorf("Greet(\"Ada\") = %q, want \"Hello, Ada!\"", got)
	}
	if got := Greet(""); got != "Hello, World!" {
		t.Errorf("Greet(\"\") = %q, want \"Hello, World!\"", got)
	}
}
