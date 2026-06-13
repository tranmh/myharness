package main

func Greet(name string) string {
	if name == "" {
		return "Hello, World!"
	}
	return "Hello, " + name + "!"
}
