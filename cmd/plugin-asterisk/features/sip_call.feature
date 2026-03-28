Feature: SIP Call Entity
  Represents an active call on the PBX.

  Scenario: Create with default state
    Given a sip_call entity "test.asterisk.call_abc" named "Active Call" with state "up" caller "100" callee "200" duration 60
    When I retrieve "test.asterisk.call_abc"
    Then the entity type is "sip_call"
    And the call state is "up"
    And the call caller is "100"
    And the call callee is "200"
    And the call duration is 60

  Scenario: Query by type
    Given a sip_call entity "test.asterisk.call1" named "Call 1" with state "up" caller "100" callee "200" duration 30
    And a sip_endpoint entity "test.asterisk.ext100" named "Ext 100" with registered true
    When I query where "type" equals "sip_call"
    Then the results include "test.asterisk.call1"
    And the results do not include "test.asterisk.ext100"

  Scenario: Update call state
    Given a sip_call entity "test.asterisk.callUpd" named "Call" with state "ringing" caller "100" callee "200" duration 0
    And I update call "test.asterisk.callUpd" to state "up"
    When I retrieve "test.asterisk.callUpd"
    Then the call state is "up"

  Scenario: Delete removes entity
    Given a sip_call entity "test.asterisk.callDel" named "Call" with state "up" caller "100" callee "200" duration 10
    When I delete "test.asterisk.callDel"
    Then retrieving "test.asterisk.callDel" should fail

  Scenario: Command sip_hangup dispatches
    Given a command listener on "test.>"
    When I send "sip_hangup" to "test.asterisk.call_abc"
    Then the received command action is "sip_hangup"

  Scenario: Command sip_transfer dispatches
    Given a command listener on "test.>"
    When I send "sip_transfer" with extension "300" to "test.asterisk.call_abc"
    Then the received command action is "sip_transfer"

  Scenario: Raw payload decodes to canonical state
    When I decode a "sip_call" payload '{"state":"up","caller":"100","callee":"200","duration":120}'
    Then the call state is "up"
    And the call caller is "100"
    And the call callee is "200"
    And the call duration is 120

  Scenario: Internal data is stored and hidden from queries
    Given a sip_call entity "test.asterisk.call_abc" named "Call" with state "up" caller "100" callee "200" duration 30
    And I write internal data for "test.asterisk.call_abc" with payload '{"channel":"SIP/100-00000001","uniqueid":"1234567890.1"}'
    When I read internal data for "test.asterisk.call_abc"
    Then the internal data matches '{"channel":"SIP/100-00000001","uniqueid":"1234567890.1"}'
    And querying type "sip_call" returns only state entities
