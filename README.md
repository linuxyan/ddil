# ddil
diff docker images layers


# build && install

go build -o ddil ddil.go && mv ddil /usr/local/bin/

# Usage 

ddil <image1_url_old> <image2_url_new>
