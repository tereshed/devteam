package main

import (
	"fmt"
	"net/url"
	"github.com/google/uuid"
	"github.com/gin-gonic/gin/binding"
    "net/http"
)

type Req struct {
	TeamID          *uuid.UUID `form:"team_id"`
	AssignedAgentID *uuid.UUID `form:"assigned_agent_id"`
}

func main() {
	var req Req
	values := url.Values{"assigned_agent_id": []string{"0456408b-8745-4986-a392-37a1a731806f"}}
    request, _ := http.NewRequest("GET", "/?assigned_agent_id=0456408b-8745-4986-a392-37a1a731806f", nil)
    request.Form = values
	err := binding.Form.Bind(request, &req) 
	fmt.Println(err)
}
