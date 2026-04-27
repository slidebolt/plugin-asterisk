Feature: PBX Entity
  The PBX entity represents the Asterisk server itself.

  Scenario: Create with default state
    Given a pbx entity "test.server.status" named "Asterisk PBX" with connected false
    When I retrieve "test.server.status"
    Then the entity type is "pbx"
    And the pbx connected is false

  Scenario: State fields hydrate correctly
    Given a pbx entity "test.server.status" named "Asterisk PBX" with connected true version "20.5.0" uptime 86400
    When I retrieve "test.server.status"
    Then the pbx connected is true
    And the pbx version is "20.5.0"
    And the pbx uptime is 86400

  Scenario: Query by type
    Given a pbx entity "test.server.status" named "Asterisk PBX" with connected true
    And a sip_trunk entity "test.trunk1.registration" named "Trunk" with registered true host "voip.ms"
    When I query where "type" equals "pbx"
    Then the results include "test.server.status"
    And the results do not include "test.trunk1.registration"

  Scenario: Update is reflected on retrieval
    Given a pbx entity "test.server.status" named "PBX" with connected false
    And I update pbx "test.server.status" to connected true
    When I retrieve "test.server.status"
    Then the pbx connected is true

  Scenario: Delete removes entity
    Given a pbx entity "test.serverDel.status" named "PBX" with connected false
    When I delete "test.serverDel.status"
    Then retrieving "test.serverDel.status" should fail

  Scenario: Command pbx_reload dispatches
    Given a command listener on "test.>"
    When I send "pbx_reload" to "test.server.status"
    Then the received command action is "pbx_reload"

  Scenario: Raw payload decodes to canonical state
    When I decode a "pbx" payload '{"connected":true,"version":"20.5.0","uptime":86400}'
    Then the pbx connected is true
    And the pbx version is "20.5.0"

  Scenario: Encode pbx_reload produces wire format
    When I encode "pbx_reload" command with '{}'
    Then the wire payload field "action" equals "Reload"

  Scenario: Internal data is stored and hidden from queries
    Given a pbx entity "test.server.status" named "PBX" with connected true
    And I write internal data for "test.server.status" with payload '{"ariEndpoint":"http://asterisk.example:8088"}'
    When I read internal data for "test.server.status"
    Then the internal data matches '{"ariEndpoint":"http://asterisk.example:8088"}'
    And querying type "pbx" returns only state entities
