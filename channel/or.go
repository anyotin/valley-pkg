package channel

func OrDone(channels ...<-chan interface{}) <-chan interface{} {
	switch len(channels) {
	case 0:
		// untyped nil は、 「chan / map / func / pointer / slice / interface」のような
		//「nil を値として持てる型」に そのまま代入可能。
		return nil
	case 1:
		return channels[0]
	}

	orDone := make(chan interface{})
	go func() {
		defer close(orDone)

		switch len(channels) {
		case 2:
			select {
			case <-channels[0]:
			case <-channels[1]:
			}
		default:
			select {
			case <-channels[1]:
			case <-channels[2]:
			case <-OrDone(append(channels[3:], orDone)...):
			}
		}
	}()

	return orDone
}
