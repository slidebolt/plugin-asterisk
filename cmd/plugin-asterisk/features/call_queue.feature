Feature: Call Queue Entity
  Represents an Asterisk call queue.

  Scenario: Create with default state
    Given a call_queue entity "test.queue_support.stats" named "Support Queue" with callers 5 available 2 strategy "ringall"
    When I retrieve "test.queue_support.stats"
    Then the entity type is "call_queue"
    And the queue callers is 5
    And the queue available is 2
    And the queue strategy is "ringall"

  Scenario: Query by type
    Given a call_queue entity "test.queue1.stats" named "Queue 1" with callers 3 available 1 strategy "ringall"
    And a sip_endpoint entity "test.ext100.registration" named "Ext 100" with registered true
    When I query where "type" equals "call_queue"
    Then the results include "test.queue1.stats"
    And the results do not include "test.ext100.registration"

  Scenario: Delete removes entity
    Given a call_queue entity "test.queueDel.stats" named "Queue" with callers 0 available 0 strategy "ringall"
    When I delete "test.queueDel.stats"
    Then retrieving "test.queueDel.stats" should fail

  Scenario: Raw payload decodes to canonical state
    When I decode a "call_queue" payload '{"callers":3,"available":2,"strategy":"leastrecent","holdtime":30}'
    Then the queue callers is 3
    And the queue available is 2
    And the queue strategy is "leastrecent"

  Scenario: Internal data is stored and hidden from queries
    Given a call_queue entity "test.queue_support.stats" named "Queue" with callers 0 available 3 strategy "ringall"
    And I write internal data for "test.queue_support.stats" with payload '{"maxWait":300,"wrapUpTime":15}'
    When I read internal data for "test.queue_support.stats"
    Then the internal data matches '{"maxWait":300,"wrapUpTime":15}'
    And querying type "call_queue" returns only state entities
