package runtime

// Assert assert
func Assert(err error) {
	if err != nil {
		panic(err)
	}
}
