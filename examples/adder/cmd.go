package adder

import (
	"fmt"
	"strconv"

	"github.com/ipfs/go-ipfs-cmdkit"
	"gx/ipfs/QmezbW7VUAiu3aSV6r4TdB9pwficnnbtWYKRsoEKF2w8G2/go-ipfs-cmds"
)

// Define the root of the commands
var RootCmd = &cmds.Command{
	Subcommands: map[string]*cmds.Command{
		"add": &cmds.Command{
			Arguments: []cmdkit.Argument{
				cmdkit.StringArg("summands", true, true, "values that are supposed to be summed"),
			},
			Run: func(req cmds.Request, re cmds.ResponseEmitter) {
				sum := 0

				for i, str := range req.Arguments() {
					num, err := strconv.Atoi(str)
					if err != nil {
						re.SetError(err, cmdkit.ErrNormal)
						return
					}

					sum += num
					re.Emit(fmt.Sprintf("intermediate result: %d; %d left", sum, len(req.Arguments())-i-1))
				}

				re.Emit(fmt.Sprintf("total: %d", sum))
			},
		},
	},
}
