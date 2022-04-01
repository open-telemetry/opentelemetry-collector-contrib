# Otto Design

Otto is a single page web application that runs all of its UI in vanilla JavaScript.

## Endpoints

* `/` serves static files

* `/components` returns a JSON representation of all registered Collector components 

* `/cfgschema/{componentType}/{componentName}` returns config schema JSON for individual components, e.g. `/cfgschema/receiver/redis`
 
* `/jsonToYAML` converts JSON to YAML

* `/ws/{componentType}/{componentName}` starts the given component, puts it in a pipeline, and opens a web socket to send component output e.g. `/ws/receiver/redis`

## Operation

At startup, the browser fetches index.html, which fetches the JavaScript and CSS files. Otto then gets the registered Collector component details from
`/components` and presents a select menu with the three component types: Metrics, Logs, and Traces. If the user selects a component type, Otto
presents select menus for the receivers, processors, and exporters that support the selected component type. If the user then selects a component, Otto
fetches the config schema JSON for the selected component and presents a form to configure the component.
At this point, if the user presses the Apply button on the top-level form fieldset, Otto sends user's inputs the `/jsonToYAML` endpoint, and presents the
component YAML in a text area. At this point, the user may start the component.

If the user starts the component (for example a receiver), Otto opens a websocket via the `/ws/{componentType}/{componentName}` endpoint and then sends
the component YAML to the backend. The backend then creates the component with the given configuration, wraps it in an object that listens for its output
and attaches it to an Otto pipeline at the appropriate slot (receiver, processor, or exporter). The wrapping object sends any output produced by the component
through the websocket to the Otto JavaScript frontend, where it is presented to the user.

At any point, the user may generate the full Collector config from any components already configured by clicking the "Generate Collector YAML" button at the bottom
of the UI. This operation is handled on the client side, in JavaScript.
