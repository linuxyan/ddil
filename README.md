# ddil
diff docker images layers


# build && install

CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -o ddil ddil.go

mv ddil /usr/local/bin/

# Usage 

ddil <image1_url_old> <image2_url_new>
