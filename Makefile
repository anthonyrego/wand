FQBN   := esp32:esp32:esp32c6
SKETCH := firmware/wand_controller

.PHONY: compile upload test sim monitor view

compile:
	arduino-cli compile --fqbn $(FQBN) $(SKETCH)

upload:
	@PORT=$$(./scripts/select-port.sh) && $(MAKE) compile && arduino-cli upload --fqbn $(FQBN) --port $$PORT $(SKETCH)

test:
	go test ./...

sim:
	go run ./cmd/wandsim

monitor:
	go run ./cmd/wandtest

view:
	go run ./cmd/wandview
