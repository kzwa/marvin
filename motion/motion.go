package motion

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/eikeon/gpio"
	"github.com/nogiushi/marvin/nog"
)

var Root = ""

func init() {
	_, filename, _, _ := runtime.Caller(0)
	Root = path.Dir(filename)
}

type Motion struct {
	Motion        bool
	Switch        map[string]bool
	motionChannel <-chan bool
}

func (s *Motion) Run(in <-chan nog.Message, out chan<- nog.Message) {
	options := nog.BitOptions{Name: "Motion", Required: false}
	if what, err := json.Marshal(&options); err == nil {
		out <- nog.NewMessage("Motion", string(what), "register")
	} else {
		log.Println("StateChanged err:", err)
	}

	name := "motion.html"
	if j, err := os.OpenFile(path.Join(Root, name), os.O_RDONLY, 0666); err == nil {
		if b, err := ioutil.ReadAll(j); err == nil {
			out <- nog.NewMessage("Marvin", string(b), "template")
		} else {
			log.Println("ERROR reading:", err)
		}
	} else {
		log.Println("WARNING: could not open ", name, err)
	}

	if c, err := gpio.GPIOInterrupt(7); err == nil {
		s.motionChannel = c
	} else {
		log.Println("Warning: Motion sensor off:", err)
		out <- nog.NewMessage("Marvin", "no motion sensor found", "Motion")
		close(out)
		return
	}
	var motionTimer *time.Timer
	var motionTimeout <-chan time.Time

	for {
		select {
		case m := <-in:
			if m.Why == "statechanged" {
				dec := json.NewDecoder(strings.NewReader(m.What))
				if err := dec.Decode(s); err != nil {
					log.Println("motion decode err:", err)
				}
			}
		case motion := <-s.motionChannel:
			if motion {
				out <- nog.NewMessage("Marvin", "motion detected", "Motion")
				if s.Switch["Nightlights"] {
					out <- nog.NewMessage("Marvin", "do nightlights on", "motion detected")
				}
				const duration = 60 * time.Second
				if motionTimer == nil {
					s.Motion = true
					motionTimer = time.NewTimer(duration)
					motionTimeout = motionTimer.C // enable motionTimeout case
				} else {
					motionTimer.Reset(duration)
				}
			}
		case <-motionTimeout:
			s.Motion = false
			motionTimer = nil
			motionTimeout = nil
			if s.Switch["Nightlights"] {
				out <- nog.NewMessage("Marvin", "set light All to off", "motion timeout")
			}
		}
	}
}

func (m *Motion) MotionSensor() bool {
	return m.motionChannel != nil
}
