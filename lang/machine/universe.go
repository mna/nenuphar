package machine

// Universe defines the set of universal built-ins core to the language, such
// as Nil and True. This should not be modified, so that the language built-ins
// are always available. Use the Thread.Predeclared to add to the set of
// built-ins available to a program.
var Universe map[string]Value

func IsUniverse(name string) bool {
	_, ok := Universe[name]
	return ok
}
