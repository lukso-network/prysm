package beacon

var errInts = errors.New("error in subscribeInts")

func subscribeMinimalEpochConsensusInfo(max, fail MinimalEpochConsensusInfo, c chan<- int) Subscription {
	return NewSubscription(func(quit <-chan struct{}) error {

		return nil
	})
}

func TestNewSubscriptionError(t *testing.T) {
	t.Parallel()

	exampleMinimalConsensusInfo := MinimalEpochConsensusInfo{

	}

	channel := make(chan int)
	sub := subscribeMinimalEpochConsensusInfo(10, 2, channel)
loop:
	for want := 0; want < 10; want++ {
		select {
		case got := <-channel:
			require.Equal(t, want, got)
		case err := <-sub.Err():
			require.Equal(t, errInts, err)
			require.Equal(t, 2, want)
			break loop
		}
	}
	sub.Unsubscribe()

	err, ok := <-sub.Err()
	require.NoError(t, err)
	if ok {
		t.Fatal("channel still open after Unsubscribe")
	}
}