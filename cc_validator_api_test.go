package cc_validator_api_test

import (
	"fmt"
	"testing"
	"time"

	api "cc_validator_api"
)

func TestCanReadCard(t *testing.T) {
	c, er := api.NewConnection("COM4", api.Baud9600)

	//fmt.Println(r)
	if er != nil {
		fmt.Println(er)
		return
	}

	status, _, er := c.Poll() // get status

	if status == api.UnitDisabled {

	}

	_, er = c.Identification()
	_, er = c.GetBillTable()
	status, _, er = c.Poll() // initialization

	if er != nil {
		fmt.Println(er)
		return
	}
	if status != api.Idling {
		fmt.Println("can not initialize")
		return
	}

	for {
		time.Sleep(time.Second * 1)
		status, param, e := c.Poll()

		if e != nil {
			fmt.Println(e)
			break
		}

		fmt.Sprintf("%X %X", status, param)
	}

}
