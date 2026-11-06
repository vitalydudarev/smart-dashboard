.PHONY: dev

# Run backend and frontend together; Ctrl+C stops both.
dev:
	@trap 'kill 0' INT TERM EXIT; \
	go -C backend run . & \
	npm run dev --prefix frontend & \
	wait
