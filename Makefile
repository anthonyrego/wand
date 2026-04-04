FQBN   := esp32:esp32:esp32c6
PORT   := /dev/cu.usbmodem101
SKETCH := firmware/wand_controller

.PHONY: compile upload test sim monitor

compile:
	arduino-cli compile --fqbn $(FQBN) $(SKETCH)

upload: compile
	arduino-cli upload --fqbn $(FQBN) --port $(PORT) $(SKETCH)

test:
	go test ./...

sim:
	go run ./cmd/wandsim

monitor:
	go run ./cmd/wandtest
