Feature: Voicemail Entity
  Represents a voicemail box on the PBX.

  Scenario: Create with default state
    Given a voicemail entity "test.vm_100.mailbox" named "VM 100" with new_messages 3 old_messages 7 mailbox "100"
    When I retrieve "test.vm_100.mailbox"
    Then the entity type is "voicemail"
    And the voicemail new_messages is 3
    And the voicemail old_messages is 7
    And the voicemail mailbox is "100"

  Scenario: Query by type
    Given a voicemail entity "test.vm100.mailbox" named "VM 100" with new_messages 1 old_messages 0 mailbox "100"
    And a sip_endpoint entity "test.ext100.registration" named "Ext 100" with registered true
    When I query where "type" equals "voicemail"
    Then the results include "test.vm100.mailbox"
    And the results do not include "test.ext100.registration"

  Scenario: Query voicemail with new messages
    Given a voicemail entity "test.vm100.mailbox" named "VM 100" with new_messages 3 old_messages 0 mailbox "100"
    And a voicemail entity "test.vm200.mailbox" named "VM 200" with new_messages 0 old_messages 5 mailbox "200"
    When I query where "type" equals "voicemail" and "state.new_messages" greater than 0
    Then I get 1 result
    And the results include "test.vm100.mailbox"

  Scenario: Update is reflected on retrieval
    Given a voicemail entity "test.vmUpd.mailbox" named "VM" with new_messages 0 old_messages 0 mailbox "100"
    And I update voicemail "test.vmUpd.mailbox" to new_messages 2 old_messages 1
    When I retrieve "test.vmUpd.mailbox"
    Then the voicemail new_messages is 2
    And the voicemail old_messages is 1

  Scenario: Delete removes entity
    Given a voicemail entity "test.vmDel.mailbox" named "VM" with new_messages 0 old_messages 0 mailbox "100"
    When I delete "test.vmDel.mailbox"
    Then retrieving "test.vmDel.mailbox" should fail

  Scenario: Command voicemail_delete dispatches
    Given a command listener on "test.>"
    When I send "voicemail_delete" with mailbox "100" to "test.vm_100.mailbox"
    Then the received command action is "voicemail_delete"

  Scenario: Raw payload decodes to canonical state
    When I decode a "voicemail" payload '{"new_messages":5,"old_messages":10,"mailbox":"100"}'
    Then the voicemail new_messages is 5
    And the voicemail old_messages is 10
    And the voicemail mailbox is "100"

  Scenario: Internal data is stored and hidden from queries
    Given a voicemail entity "test.vm_100.mailbox" named "VM" with new_messages 0 old_messages 0 mailbox "100"
    And I write internal data for "test.vm_100.mailbox" with payload '{"vmContext":"default","vmPassword":"1234"}'
    When I read internal data for "test.vm_100.mailbox"
    Then the internal data matches '{"vmContext":"default","vmPassword":"1234"}'
    And querying type "voicemail" returns only state entities
