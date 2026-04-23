FQBN     := esp32:esp32:esp32c6
SKETCH   := firmware/wand_controller
LIBS_DIR := firmware/libraries

.PHONY: compile upload test sim monitor play

compile:
	arduino-cli compile --fqbn $(FQBN) --libraries $(LIBS_DIR) $(SKETCH)

upload:
	@PORT=$$(./scripts/select-port.sh) && $(MAKE) compile && arduino-cli upload --fqbn $(FQBN) --port $$PORT $(SKETCH)

test:
	go test ./...

sim:
	go run ./cmd/wandsim

monitor:
	go run ./cmd/wandtest

play:
	go run ./cmd/play
