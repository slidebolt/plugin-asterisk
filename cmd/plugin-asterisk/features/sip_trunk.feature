Feature: SIP Trunk Entity
  Represents a SIP trunk connection (e.g. VoIP.ms).

  Scenario: Create with default state
    Given a sip_trunk entity "test.trunk_voipms.registration" named "VoIP.ms" with registered false host "chicago3.voip.ms"
    When I retrieve "test.trunk_voipms.registration"
    Then the entity type is "sip_trunk"
    And the trunk registered is false
    And the trunk host is "chicago3.voip.ms"

  Scenario: State fields hydrate correctly
    Given a sip_trunk entity "test.trunk_voipms.registration" named "VoIP.ms" with registered true host "chicago3.voip.ms" port 5060 latency 25
    When I retrieve "test.trunk_voipms.registration"
    Then the trunk registered is true
    And the trunk host is "chicago3.voip.ms"
    And the trunk latency is 25

  Scenario: Query by type
    Given a sip_trunk entity "test.trunk1.registration" named "Trunk 1" with registered true host "sip.example.com"
    And a sip_endpoint entity "test.ext100.registration" named "Ext 100" with registered true
    When I query where "type" equals "sip_trunk"
    Then the results include "test.trunk1.registration"
    And the results do not include "test.ext100.registration"

  Scenario: Query registered trunks
    Given a sip_trunk entity "test.trunk1.registration" named "Trunk 1" with registered true host "sip1.example.com"
    And a sip_trunk entity "test.trunk2.registration" named "Trunk 2" with registered false host "sip2.example.com"
    When I query where "type" equals "sip_trunk" and "state.registered" equals "true"
    Then I get 1 result
    And the results include "test.trunk1.registration"

  Scenario: Update is reflected on retrieval
    Given a sip_trunk entity "test.trunkUpd.registration" named "Trunk" with registered false host "sip.example.com"
    And I update trunk "test.trunkUpd.registration" to registered true
    When I retrieve "test.trunkUpd.registration"
    Then the trunk registered is true

  Scenario: Delete removes entity
    Given a sip_trunk entity "test.trunkDel.registration" named "Trunk" with registered false host "sip.example.com"
    When I delete "test.trunkDel.registration"
    Then retrieving "test.trunkDel.registration" should fail

  Scenario: Raw payload decodes to canonical state
    When I decode a "sip_trunk" payload '{"registered":true,"host":"chicago3.voip.ms","port":5060}'
    Then the trunk registered is true
    And the trunk host is "chicago3.voip.ms"

  Scenario: Internal data is stored and hidden from queries
    Given a sip_trunk entity "test.trunk_voipms.registration" named "Trunk" with registered true host "voip.ms"
    And I write internal data for "test.trunk_voipms.registration" with payload '{"peerName":"voipms","qualify":"yes"}'
    When I read internal data for "test.trunk_voipms.registration"
    Then the internal data matches '{"peerName":"voipms","qualify":"yes"}'
    And querying type "sip_trunk" returns only state entities
