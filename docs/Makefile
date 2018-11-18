all:
	docker build -t aglio .
	docker run -v $(CURDIR):/tmp -t aglio -i /tmp/api.apib -o /tmp/api.html
