package testkit

func Panic(call func()) (value any) {
	defer func() { value = recover() }()
	call()
	return nil
}
