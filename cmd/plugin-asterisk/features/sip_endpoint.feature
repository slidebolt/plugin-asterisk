Feature: SIP Endpoint Entity
  Represents a SIP extension/phone.

  Scenario: Create with default state
    Given a sip_endpoint entity "test.asterisk.ext_100" named "Extension 100" with registered false
    When I retrieve "test.asterisk.ext_100"
    Then the entity type is "sip_endpoint"
    And the endpoint registered is false

  Scenario: State fields hydrate correctly
    Given a sip_endpoint entity "test.asterisk.ext_100" named "Extension 100" with registered true in_call false ip "192.168.88.50"
    When I retrieve "test.asterisk.ext_100"
    Then the endpoint registered is true
    And the endpoint in_call is false
    And the endpoint ip is "192.168.88.50"

  Scenario: Query by type
    Given a sip_endpoint entity "test.asterisk.ext100" named "Ext 100" with registered true
    And a sip_trunk entity "test.asterisk.trunk1" named "Trunk" with registered true host "sip.example.com"
    When I query where "type" equals "sip_endpoint"
    Then the results include "test.asterisk.ext100"
    And the results do not include "test.asterisk.trunk1"

  Scenario: Query endpoints in call
    Given a sip_endpoint entity "test.asterisk.ext100" named "Ext 100" with registered true in_call true ip "192.168.88.50"
    And a sip_endpoint entity "test.asterisk.ext200" named "Ext 200" with registered true in_call false ip "192.168.88.51"
    When I query where "type" equals "sip_endpoint" and "state.in_call" equals "true"
    Then I get 1 result
    And the results include "test.asterisk.ext100"

  Scenario: Update is reflected on retrieval
    Given a sip_endpoint entity "test.asterisk.extUpd" named "Ext" with registered true
    And I update endpoint "test.asterisk.extUpd" to in_call true
    When I retrieve "test.asterisk.extUpd"
    Then the endpoint in_call is true

  Scenario: Delete removes entity
    Given a sip_endpoint entity "test.asterisk.extDel" named "Ext" with registered true
    When I delete "test.asterisk.extDel"
    Then retrieving "test.asterisk.extDel" should fail

  Scenario: Command sip_call dispatches
    Given a command listener on "test.>"
    When I send "sip_call" with extension "200" to "test.asterisk.ext_100"
    Then the received command action is "sip_call"

  Scenario: Command sip_hangup dispatches
    Given a command listener on "test.>"
    When I send "sip_hangup" to "test.asterisk.ext_100"
    Then the received command action is "sip_hangup"

  Scenario: Raw payload decodes to canonical state
    When I decode a "sip_endpoint" payload '{"registered":true,"in_call":false,"ip":"192.168.88.50"}'
    Then the endpoint registered is true
    And the endpoint in_call is false
    And the endpoint ip is "192.168.88.50"

  Scenario: Encode sip_call produces wire format
    When I encode "sip_call" command with '{"extension":"200","context":"internal"}'
    Then the wire payload field "extension" equals "200"

  Scenario: Internal data is stored and hidden from queries
    Given a sip_endpoint entity "test.asterisk.ext_100" named "Ext 100" with registered true
    And I write internal data for "test.asterisk.ext_100" with payload '{"aorContact":"sip:100@192.168.88.50:5060"}'
    When I read internal data for "test.asterisk.ext_100"
    Then the internal data matches '{"aorContact":"sip:100@192.168.88.50:5060"}'
    And querying type "sip_endpoint" returns only state entities
