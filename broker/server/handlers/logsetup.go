/*
 * Copyright 2018, CS Systemes d'Information, http://www.c-s.fr
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package handlers

import (
	"fmt"
	"io"
	"os"

	"github.com/CS-SI/SafeScale/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&utils.MyFormatter{TextFormatter: log.TextFormatter{ForceColors: true, TimestampFormat: "2006-01-02 15:04:05", FullTimestamp: true, DisableLevelTruncation: true}})
	log.SetLevel(log.DebugLevel)

	// Log as JSON instead of the default ASCII formatter.
	// log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	_ = os.MkdirAll(utils.AbsPathify("$HOME/.safescale"), 0777)
	file, err := os.OpenFile(utils.AbsPathify("$HOME/.safescale/brokerd-session.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}

	log.SetOutput(io.MultiWriter(os.Stdout, file))
}

// throwErr throws error as-is, without change.
// Used as a way to tell other developers not to alter the error.
func throwErr(err error) error {
	return err
}

func throwErrf(format string, a ...interface{}) error {
	return fmt.Errorf(format, a...)
}

// infraErr throws error with stack trace
func infraErr(err error) error {
	if err == nil {
		return nil
	}
	log.Errorf("%+v", err)

	tbr := errors.WithStack(err)
	return tbr
}

// infraErrf throws error with stack trace and adds message
func infraErrf(err error, message string, a ...interface{}) error {
	if err == nil {
		return nil
	}

	tbr := errors.WithStack(err)
	tbr = errors.WithMessage(tbr, fmt.Sprintf(message, a...))

	log.Errorf("%+v", tbr)
	return tbr
}

// logicErr ...
func logicErr(err error) error {
	if err == nil {
		return nil
	}
	log.Errorf("%+v", err)
	return err
}

// logicErrf ...
func logicErrf(err error, message string, a ...interface{}) error {
	if err == nil {
		return nil
	}
	tbr := errors.Wrap(err, fmt.Sprintf(message, a...))
	log.Errorf("%+v", tbr)
	return tbr
}