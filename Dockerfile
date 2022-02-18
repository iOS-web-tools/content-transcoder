# golang image
FROM arm64v8/golang AS build

# set work dir
WORKDIR /app

# copy the source files
COPY . .

# disable crosscompiling 
ENV CGO_ENABLED=0

# compile linux only
ENV GOOS=linux

# build the binary with debug information removed
RUN go build -mod=vendor -ldflags '-w -s' -a -installsuffix cgo -o server

FROM arm64v8/alpine

# install additional dependencies for ffmpeg
RUN apk add --no-cache --update libgcc libstdc++ ca-certificates libcrypto1.1 libssl1.1 libgomp expat

# copy our static linked library
COPY --from=build /app/server .

# make output dir
RUN mkdir ./out

# tell we are exposing our service on port 8080 and 8081
EXPOSE 8080 8081

# run it!
CMD ["./server"]
