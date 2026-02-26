.PHONY: models docker-build docker-run clean

models:
	mkdir -p models
	curl -fsL -o models/detect.prototxt https://github.com/WeChatCV/opencv_3rdparty/raw/wechat_qrcode/detect.prototxt
	curl -fsL -o models/detect.caffemodel https://github.com/WeChatCV/opencv_3rdparty/raw/wechat_qrcode/detect.caffemodel
	curl -fsL -o models/sr.prototxt https://github.com/WeChatCV/opencv_3rdparty/raw/wechat_qrcode/sr.prototxt
	curl -fsL -o models/sr.caffemodel https://github.com/WeChatCV/opencv_3rdparty/raw/wechat_qrcode/sr.caffemodel

docker-build: models
	docker build -t qr-scanner .

docker-run:
	docker run --rm -v $(PWD)/images:/app/images qr-scanner /app/images

clean:
	rm -rf models
	rm -f qr-scanner

deploy-fuzzer:
	@if [ -z "$(HOST)" ]; then echo "Error: Please specify HOST. Example: make deploy-fuzzer HOST=user@server"; exit 1; fi
	@echo "Building standalone Fuzzer image..."
	docker build -f Dockerfile.standalone -t qr-scan-fuzzer:latest .
	@echo "Exporting Docker image to tar archive..."
	docker save qr-scan-fuzzer:latest | gzip > fuzzer_image.tar.gz
	@echo "Transferring image and config to $(HOST)..."
	scp fuzzer_image.tar.gz docker-compose.server.yml $(HOST):~/
	@echo "Loading image on the remote server..."
	ssh $(HOST) 'docker load < ~/fuzzer_image.tar.gz && rm ~/fuzzer_image.tar.gz'
	@echo "Done! Connect to $(HOST) and run: docker compose -f docker-compose.server.yml up -d"
