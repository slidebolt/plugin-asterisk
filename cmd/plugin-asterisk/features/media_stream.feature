Feature: Media Stream Entity
  Represents a stream served by sb-media. The plugin owns the entity and the
  source, and clients read the URL from its state.

  Scenario: Create with default state
    Given a media_stream entity "test.ext_100.audio" named "Audio" with status "idle" kind "audio"
    When I retrieve "test.ext_100.audio"
    Then the entity type is "media_stream"
    And the stream status is "idle"
    And the stream kind is "audio"
    And the stream viewers is 0

  Scenario: State fields hydrate correctly
    Given a media_stream entity "test.ext_100.audio" named "Audio" with status "live" kind "audio" url "http://media/m/x/stream.ts" format "mpegts" viewers 2
    When I retrieve "test.ext_100.audio"
    Then the stream status is "live"
    And the stream url is "http://media/m/x/stream.ts"
    And the stream format is "mpegts"
    And the stream viewers is 2

  Scenario: Query by type
    Given a media_stream entity "test.ext100.audio" named "Audio" with status "idle" kind "audio"
    And a sip_endpoint entity "test.ext100.registration" named "Ext 100" with registered true
    When I query where "type" equals "media_stream"
    Then the results include "test.ext100.audio"
    And the results do not include "test.ext100.registration"

  Scenario: Query live streams
    Given a media_stream entity "test.ext100.audio" named "Audio" with status "live" kind "audio"
    And a media_stream entity "test.ext200.audio" named "Audio" with status "idle" kind "audio"
    When I query where "type" equals "media_stream" and "state.status" equals "live"
    Then I get 1 result
    And the results include "test.ext100.audio"

  Scenario: Update is reflected on retrieval
    Given a media_stream entity "test.extUpd.audio" named "Audio" with status "idle" kind "audio"
    And I update stream "test.extUpd.audio" to status "live"
    When I retrieve "test.extUpd.audio"
    Then the stream status is "live"

  Scenario: Delete removes entity
    Given a media_stream entity "test.extDel.audio" named "Audio" with status "idle" kind "audio"
    When I delete "test.extDel.audio"
    Then retrieving "test.extDel.audio" should fail

  Scenario: Command media_stream_start dispatches
    Given a command listener on "test.>"
    When I send "media_stream_start" to "test.ext_100.audio"
    Then the received command action is "media_stream_start"

  Scenario: Command media_stream_stop dispatches
    Given a command listener on "test.>"
    When I send "media_stream_stop" to "test.ext_100.audio"
    Then the received command action is "media_stream_stop"

  Scenario: Internal data is stored and hidden from queries
    Given a media_stream entity "test.ext_100.audio" named "Audio" with status "idle" kind "audio"
    And I write internal data for "test.ext_100.audio" with payload '{"bridgeId":"b1"}'
    When I read internal data for "test.ext_100.audio"
    Then the internal data matches '{"bridgeId":"b1"}'
    And querying type "media_stream" returns only state entities

  Scenario: Plugin reports a missing host instead of inventing devices
    Given the plugin is running without a host configured
    When I retrieve "plugin-asterisk.server.status"
    Then the entity type is "pbx"
    And the pbx connected is false
    And retrieving "plugin-asterisk.ext_3000.registration" should fail
