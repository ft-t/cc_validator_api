package cc_validator_api_test

import (
	api "cc_validator_api"
	"fmt"
	"testing"
	"time"
)

func TestCanReadCard(t *testing.T) {
	c, er := api.NewConnection("COM4", api.Baud9600)

	//fmt.Println(r)
	if er != nil {
		fmt.Println(er)
		return
	}


	b,er := c.Poll() // get status

	if b[0] == byte(0x19) {

	}

	b,er = c.Identification()
	b,er = c.GetBillTable()
	b, er = c.Poll() // initialization

	if er != nil {
		fmt.Println(er)
		return
	}
	if b[0] != 0x13 {
		fmt.Println("can not initialize")
		return
	}

	for {
		time.Sleep(time.Second * 1)
		b, e := c.Poll()

		if e != nil {
			fmt.Println(e)
			break
		}

		fmt.Sprintf("+%v", b)
	}

}
