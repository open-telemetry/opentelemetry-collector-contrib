# Otto Frontend Conventions

## Views

Views are [Humble Objects](https://martinfowler.com/bliki/HumbleObject.html) containing a minimal amount of logic just to manipulate the DOM.
Views inherit from the View class.

## Widgets

Widgets are just Views containing no application-specific presentation logic. They are general enough that, in theory, they could be used
by other applications without modification. 

## Controllers

Controllers contain application and presentation logic independent of the DOM. They are designed to be testable eventually, although
they will likely require changes to attain that goal.
