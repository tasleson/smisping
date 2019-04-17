package main

//
// Copyright @ 2015 Tony Asleson <tasleson@redhat.com>
// License: Apache License Version 2.0, see:
// http://www.apache.org/licenses/LICENSE-2.0.html
//
// Note: My very first exposure to golang, so this probably could be done
// much better.

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/runner-mei/gowbem"
)

func getArrayURI(urlIn, user, password string) *url.URL {

	// TODO Use a regular expression to really validate the url.
	r := strings.Split(urlIn, "://")

	if len(r) != 2 {
		bail(5, fmt.Sprintf("Invalid URL (%s)", urlIn))
	}

	scheme := "http"

	if r[0] == "https" {
		scheme = "https"
	}

	return &url.URL{
		Scheme: scheme,
		User:   url.UserPassword(user, password),
		Host:   r[1],
		Path:   "/",
	}
}

func bail(ec int, args ...interface{}) {
	fmt.Printf("%s", fmt.Sprintln(args...))
	os.Exit(ec)
}

func hostAvailable(host string) {
	// Lets try to establish a socket to the host and port for some additional
	// diagnostics
	conn, err := net.Dial("tcp", host)
	if nil != err {
		bail(4, "Host down or port not open! ", host)
	}
	conn.Close()
}

func getRps(c *gowbem.ClientCIMXML) (gowbem.CIMInstanceWithName, string, error) {

	nameSpaces := [...]string{"interop", "root/interop", "root/PG_Interop"}

	for _, ns := range nameSpaces {
		instances, e := c.EnumerateInstances(ns, "CIM_RegisteredProfile",
			false, false, false, false, nil)
		if nil != e {
			// At the moment the gowbem library just returns the XML from the
			// failed operation, we will look through it for likely causes
			// until the library can handle this better.
			if strings.Contains(e.Error(), "CIM_ERR_ACCESS_DENIED") {
				bail(2, "Incorrect credentials!")
			} else if strings.Contains(e.Error(), "CIM_ERR_INVALID_NAMESPACE") {
				// Expected error at times.
				continue
			} else {
				fmt.Printf("Enumerate instances ns=(%s) result(%s)", ns, e)
			}
			continue
		}

		if instances != nil && len(instances) > 0 {
			for _, inst := range instances {
				prop := inst.GetInstance().GetPropertyByName("RegisteredOrganization")

				v := prop.GetValue()
				t := prop.GetType()

				if t.GetType() == gowbem.UINT16 {
					tIntVal, e := strconv.Atoi(v.(string))

					if nil != e {
						bail(5, e)
					}

					if tIntVal == 11 {
						regName := inst.GetInstance().GetPropertyByName("RegisteredName")

						if nil != regName {
							if "Array" == inst.GetInstance().GetPropertyByName("RegisteredName").GetValue().(string) {
								return inst, ns, nil
							}
						}
					}
				}
			}
		}
	}

	return nil, "", errors.New("Provider does not appear to support interop or username/password incorrect")
}

func main() {
	if len(os.Args) != 4 {
		fmt.Printf("Usage: %s <url> <username> <password>\n", os.Args[0])
		fmt.Printf("   eg. %s https://127.0.0.1:5989 someuser uber_secret\n", os.Args[0])
		fmt.Printf("Source: https://github.com/tasleson/smisping\n")
		os.Exit(10)
	}

	connectionInfo := getArrayURI(os.Args[1], os.Args[2], os.Args[3])

	// Lets see if we can establish a socket to the one supplied.
	hostAvailable(connectionInfo.Host)

	c, e := gowbem.NewClientCIMXML(connectionInfo, true)

	if nil != e {
		bail(5, e)
	}

	item, namespace, e := getRps(c)
	if nil != e {
		bail(3, e)
	}

	systems, e := c.AssociatorInstances(
		namespace, item.GetName(),
		"CIM_ElementConformsToProfile", "CIM_ComputerSystem", "", "",
		false, nil)

	if nil != e {
		bail(5, e)
	}

	if len(systems) > 0 {
		fmt.Printf("Found %d system(s)\n", len(systems))
		os.Exit(0)
	}
	fmt.Printf("No systems found\n")
	os.Exit(1)
}
