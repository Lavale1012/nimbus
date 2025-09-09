package animations

// func StartSpinner(stop chan struct{}) {
// 	frames := []rune{'|', '/', '-', '\\'}
// 	ticker := time.NewTicker(100 * time.Millisecond)
// 	defer ticker.Stop()

// 	i := 0
// 	for {
// 		select {
// 		case <-stop:
// 			fmt.Printf("\r") // clear the spinner
// 			return
// 		case <-ticker.C:
// 			fmt.Printf("\rLoading... %c", frames[i%len(frames)])
// 			i++
// 		}
// 	}
// }
