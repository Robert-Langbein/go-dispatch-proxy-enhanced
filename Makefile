.PHONY: build up down restart logs clean help

# Default target
help:
	@echo "Available commands:"
	@echo "  build    - Build the Docker image"
	@echo "  up       - Start the container"
	@echo "  down     - Stop and remove the container"
	@echo "  restart  - Restart the container"
	@echo "  logs     - Show container logs"
	@echo "  clean    - Remove image and volumes"
	@echo "  rebuild  - Clean build and start"

# Build the Docker image
build:
	docker-compose build

# Start the container
up:
	docker-compose up -d

# Stop and remove the container
down:
	docker-compose down

# Restart the container
restart:
	docker-compose restart

# Show container logs
logs:
	docker-compose logs -f

# Remove everything (image, containers, volumes)
clean:
	docker-compose down -v --rmi all

# Clean build and start
rebuild: clean build up

# Quick status check
status:
	docker-compose ps 