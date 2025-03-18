package safe

func Run(fn func()) {
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		// todo log recover
	// 		slog.Error("panic", slog.Any("recover", r), slog.String("component", "safe.Run"))
	// 	}
	// }()

	fn()
}
