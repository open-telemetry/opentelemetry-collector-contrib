# Otto Usage

## Overview

Otto is a single-user web application to help you configure an OpenTelemetry Collector.

## Starting the Otto server

Otto is a self-contained executable that can be run from within a local OpenTelemetry Collector repository.

1) Download or clone a copy of the Collector.
2) Change directories to `cmd/configschema/otto`
3) Run `go run .`
4) Open localhost:8888 in a desktop browser (only Chrome is officially supported at this time, though other browsers may also work).

## Creating a component configuration

When Otto loads, it presents you with a select menu where you can choose which kind of Collector pipeline you want to create: metrics, logs, or traces.
When you select a pipeline type, Otto populates the UI with the components that support the selected pipeline type. At this point, you can select a component.

When you select a component, Otto creates a dynamic form for you to configure it. This form is pre-populated with default values, which
appear as placeholder text or a select box with the default value indicated. You can leave these blank to accept the defaults or change them to provide your
own values. If you are unsure about what a field means, or what type of data it accepts, you can hover your mouse over the name of the field to view documentation
for that field in a tooltip.

Configurable subsections appear as "configure" links in the form, which, when clicked, display a sub-form and disable the parent form's links and "Apply" button. Any
visible sub-form must be applied (by clicking the "Apply" button) before its parent form can be applied. When the top-level form is applied, your inputs are converted
into a YAML snippet for the component, which may be edited manually. At this point, you can press the "Start" button, which
will start the component in an Otto pipeline, in the same environment in which the Otto server is running.

Once a component is running, Otto will listen to its output and display the output in a table. Each row in the table corresponds to a single message, which may
comprise a significant amount of information. To see a JSON dump of all of this information, each row may be clicked, and the row's data will be displayed in full
in a tab next to the "Messages" table. To navigate back to the Messages table, click the Messages tab. 

In a similar manner, processors and exporters may be added to the pipeline as well. Otto will connect these components to the Otto pipeline, so the output of a receiver
will be sent to the input of a processor, in addition to both of them sending their messages to the Otto web application.

## Creating a Collector configuration

At any point, you may click the "Generate Collector YAML" button, which will create a Collector configuration YAML document and display it in a text area.
