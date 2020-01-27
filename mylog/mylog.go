package mylog

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Type int

type responseLogger struct {
	rw     http.ResponseWriter
	start  time.Time
	status int
	size   int
}

func (rl *responseLogger) Header() http.Header {
	return rl.rw.Header()
}

func (rl *responseLogger) Write(bytes []byte) (int, error) {
	if rl.status == 0 {
		rl.status = http.StatusOK
	}

	size, err := rl.rw.Write(bytes)

	rl.size += size

	return size, err
}

func (rl *responseLogger) WriteHeader(status int) {
	rl.status = status

	rl.rw.WriteHeader(status)
}

func (rl *responseLogger) Flush() {
	f, ok := rl.rw.(http.Flusher)

	if ok {
		f.Flush()
	}
}

type loggerHanlder struct {
	h      http.Handler
	writer io.Writer
}

func (rh loggerHanlder) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	rl := &responseLogger{rw: res, start: time.Now()}
	rh.h.ServeHTTP(rl, req)
	rh.write(rl, req)
}

func createKeyValuePairs(m map[string][]string) string {
	b := new(bytes.Buffer)
	for key, value := range m {
		fmt.Fprintf(b, "%s=\"%s\"\n", key, value)
	}
	return b.String()
}

func (rh loggerHanlder) write(rl *responseLogger, req *http.Request) {

	sBody, _ := ioutil.ReadAll(req.Body)
	statusInt, _ := strconv.Atoi(strconv.Itoa(rl.status))

	fmt.Fprintln(rh.writer, strings.Join([]string{
		"===========================request begin================================================ \n",
		"URI : " + req.RequestURI + "\n",
		"METHOD : " + req.Method + "\n",
		"FROM : " + req.RemoteAddr + "\n",
		"Headers : \n",
		createKeyValuePairs(req.Header),
		"Date : " + time.Now().String() + "\n",
		"Request Body : " + string(sBody) + "\n",
		"===========================request  end================================================== \n",
		"===========================response begin================================================ \n",
		"Status Code : " + strconv.Itoa(statusInt) + "\n",
		"Status Text : " + http.StatusText(statusInt) + "\n",
		"Headers : ",
		createKeyValuePairs(rl.Header()),
		"===========================response end================================================== \n",
	}, " "))

}

func Handler(h http.Handler, writer io.Writer) http.Handler {
	return loggerHanlder{
		h:      h,
		writer: writer,
	}
}
