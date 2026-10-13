package im

import "context"

// StopCommand implements /stop.
// It cancels the in-flight QA request for the current user+chat, allowing the
// user to abort a long-running ReAct reasoning chain without waiting for it to
// complete. If no request is in progress the command simply acknowledges.
type StopCommand struct{}

func newStopCommand() *StopCommand { return &StopCommand{} }

// Name returns the command keyword.
func (c *StopCommand) Name() string { return "stop" }

// Description returns the help text shown for /stop.
func (c *StopCommand) Description() string { return "Stop the answer in progress" }

func (c *StopCommand) Execute(_ context.Context, _ *CommandContext, _ []string) (*CommandResult, error) {
	return &CommandResult{
		Content: "✅ Requested to stop the current answer.",
		Action:  ActionStop,
	}, nil
}
