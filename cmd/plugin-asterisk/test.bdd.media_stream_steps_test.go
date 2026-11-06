//go:build bdd

package main

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/cucumber/godog"
	domain "github.com/slidebolt/sb-domain"

	"github.com/slidebolt/plugin-asterisk/app"
)

func (c *bddCtx) saveMediaStream(key, name string, state domain.MediaStream) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type: "media_stream", Name: name,
		State: state,
	})
}

func (c *bddCtx) aMediaStreamEntity(key, name, status, kind string) error {
	return c.saveMediaStream(key, name, domain.MediaStream{Status: status, Kind: kind})
}

func (c *bddCtx) aMediaStreamEntityFull(key, name, status, kind, url, format string, viewers int) error {
	return c.saveMediaStream(key, name, domain.MediaStream{Status: status, Kind: kind, URL: url, Format: format, Viewers: viewers})
}

func (c *bddCtx) updateStreamStatus(key, status string) error {
	e, err := c.getEntity(key)
	if err != nil {
		return err
	}
	st, ok := e.State.(domain.MediaStream)
	if !ok {
		return fmt.Errorf("state type: got %T, want domain.MediaStream", e.State)
	}
	st.Status = status
	e.State = st
	return c.saveEntity(e)
}

func (c *bddCtx) pluginRunningWithoutHost() error {
	c.t.Setenv("ASTERISK_HOST_URL", "")
	c.t.Setenv("ASTERISK_HOST_TOKEN", "")
	a := app.New()
	deps := map[string]json.RawMessage{"messenger": c.env.MessengerPayload()}
	if _, err := a.OnStart(deps); err != nil {
		return fmt.Errorf("OnStart must not fail: %w", err)
	}
	c.t.Cleanup(func() { _ = a.OnShutdown() })
	return nil
}

func (c *bddCtx) lastStream() (domain.MediaStream, error) {
	if c.lastGetErr != nil {
		return domain.MediaStream{}, fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st, ok := c.lastEntity.State.(domain.MediaStream)
	if !ok {
		return domain.MediaStream{}, fmt.Errorf("state type: got %T, want domain.MediaStream", c.lastEntity.State)
	}
	return st, nil
}

func (c *bddCtx) checkStreamField(field string, get func(domain.MediaStream) any, expected any) error {
	st, err := c.lastStream()
	if err != nil {
		return err
	}
	if got := get(st); got != expected {
		return fmt.Errorf("stream %s: got %v, want %v", field, got, expected)
	}
	return nil
}

func (c *bddCtx) streamStatusIs(expected string) error {
	return c.checkStreamField("status", func(s domain.MediaStream) any { return s.Status }, expected)
}

func (c *bddCtx) streamKindIs(expected string) error {
	return c.checkStreamField("kind", func(s domain.MediaStream) any { return s.Kind }, expected)
}

func (c *bddCtx) streamURLIs(expected string) error {
	return c.checkStreamField("url", func(s domain.MediaStream) any { return s.URL }, expected)
}

func (c *bddCtx) streamFormatIs(expected string) error {
	return c.checkStreamField("format", func(s domain.MediaStream) any { return s.Format }, expected)
}

func (c *bddCtx) streamViewersIs(expected int) error {
	return c.checkStreamField("viewers", func(s domain.MediaStream) any { return s.Viewers }, expected)
}

func (c *bddCtx) streamErrorIs(expected string) error {
	return c.checkStreamField("error", func(s domain.MediaStream) any { return s.Error }, expected)
}

func (c *bddCtx) entityHasCommand(action string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	if !slices.Contains(c.lastEntity.Commands, action) {
		return fmt.Errorf("commands %v do not include %q", c.lastEntity.Commands, action)
	}
	return nil
}

func (c *bddCtx) registerMediaStreamSteps(ctx *godog.ScenarioContext) {
	ctx.Step(`^a media_stream entity "([^"]*)" named "([^"]*)" with status "([^"]*)" kind "([^"]*)"$`, c.aMediaStreamEntity)
	ctx.Step(`^a media_stream entity "([^"]*)" named "([^"]*)" with status "([^"]*)" kind "([^"]*)" url "([^"]*)" format "([^"]*)" viewers (\d+)$`, c.aMediaStreamEntityFull)
	ctx.Step(`^I update stream "([^"]*)" to status "([^"]*)"$`, c.updateStreamStatus)
	ctx.Step(`^the plugin is running without a host configured$`, c.pluginRunningWithoutHost)
	ctx.Step(`^the stream status is "([^"]*)"$`, c.streamStatusIs)
	ctx.Step(`^the stream kind is "([^"]*)"$`, c.streamKindIs)
	ctx.Step(`^the stream url is "([^"]*)"$`, c.streamURLIs)
	ctx.Step(`^the stream format is "([^"]*)"$`, c.streamFormatIs)
	ctx.Step(`^the stream viewers is (\d+)$`, c.streamViewersIs)
	ctx.Step(`^the stream error is "([^"]*)"$`, c.streamErrorIs)
	ctx.Step(`^the entity has command "([^"]*)"$`, c.entityHasCommand)
}
