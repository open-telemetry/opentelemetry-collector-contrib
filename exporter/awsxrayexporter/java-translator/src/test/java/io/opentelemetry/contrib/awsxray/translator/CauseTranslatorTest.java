// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator;

import static org.assertj.core.api.Assertions.assertThat;

import io.opentelemetry.contrib.awsxray.translator.model.CauseData;
import java.util.List;
import org.junit.jupiter.api.Test;

class CauseTranslatorTest {

    @Test
    void testJavaStacktraceParsing() {
        String stacktrace = "java.lang.NullPointerException: Something went wrong\n"
                + "\tat com.example.MyClass.myMethod(MyClass.java:42)\n"
                + "\tat com.example.OtherClass.call(OtherClass.java:100)\n"
                + "\tat java.base/java.lang.Thread.run(Thread.java:829)\n";

        List<CauseData.ExceptionData> exceptions = CauseTranslator.parseException(
                "java.lang.NullPointerException", "Something went wrong",
                stacktrace, false, "java");

        assertThat(exceptions).hasSize(1);
        CauseData.ExceptionData exception = exceptions.get(0);
        assertThat(exception.getType()).isEqualTo("java.lang.NullPointerException");
        assertThat(exception.getMessage()).isEqualTo("Something went wrong");
        assertThat(exception.getStack()).hasSize(3);
        assertThat(exception.getStack().get(0).getPath()).isEqualTo("MyClass.java");
        assertThat(exception.getStack().get(0).getLabel()).isEqualTo("com.example.MyClass.myMethod");
        assertThat(exception.getStack().get(0).getLine()).isEqualTo(42);
    }

    @Test
    void testJavaStacktraceWithCausedBy() {
        String stacktrace = "java.lang.RuntimeException: Outer\n"
                + "\tat com.example.Outer.run(Outer.java:10)\n"
                + "Caused by: java.io.IOException: Inner cause\n"
                + "\tat com.example.Inner.read(Inner.java:20)\n";

        List<CauseData.ExceptionData> exceptions = CauseTranslator.parseException(
                "java.lang.RuntimeException", "Outer",
                stacktrace, false, "java");

        assertThat(exceptions).hasSize(2);
        assertThat(exceptions.get(0).getType()).isEqualTo("java.lang.RuntimeException");
        assertThat(exceptions.get(0).getStack()).hasSize(1);
        assertThat(exceptions.get(0).getCause()).isEqualTo(exceptions.get(1).getId());

        assertThat(exceptions.get(1).getType()).isEqualTo("java.io.IOException");
        assertThat(exceptions.get(1).getMessage()).isEqualTo("Inner cause");
        assertThat(exceptions.get(1).getStack()).hasSize(1);
        assertThat(exceptions.get(1).getStack().get(0).getPath()).isEqualTo("Inner.java");
        assertThat(exceptions.get(1).getStack().get(0).getLine()).isEqualTo(20);
    }

    @Test
    void testJavaStacktraceWithModulePrefix() {
        String stacktrace = "java.lang.NullPointerException: oops\n"
                + "\tat java.base/java.util.Objects.requireNonNull(Objects.java:209)\n";

        List<CauseData.ExceptionData> exceptions = CauseTranslator.parseException(
                "java.lang.NullPointerException", "oops",
                stacktrace, false, "java");

        assertThat(exceptions.get(0).getStack()).hasSize(1);
        assertThat(exceptions.get(0).getStack().get(0).getLabel()).isEqualTo("java.util.Objects.requireNonNull");
    }

    @Test
    void testPythonStacktraceParsing() {
        String stacktrace = "Traceback (most recent call last):\n"
                + "  File \"/app/main.py\", line 10, in run\n"
                + "    result = compute()\n"
                + "  File \"/app/compute.py\", line 5, in compute\n"
                + "    return 1/0\n"
                + "ZeroDivisionError: division by zero\n";

        List<CauseData.ExceptionData> exceptions = CauseTranslator.parseException(
                "ZeroDivisionError", "division by zero",
                stacktrace, false, "python");

        assertThat(exceptions).hasSize(1);
        assertThat(exceptions.get(0).getStack()).hasSize(2);
        assertThat(exceptions.get(0).getStack().get(0).getPath()).isEqualTo("/app/compute.py");
        assertThat(exceptions.get(0).getStack().get(0).getLine()).isEqualTo(5);
        assertThat(exceptions.get(0).getStack().get(0).getLabel()).isEqualTo("compute");
        assertThat(exceptions.get(0).getStack().get(1).getPath()).isEqualTo("/app/main.py");
        assertThat(exceptions.get(0).getStack().get(1).getLine()).isEqualTo(10);
    }

    @Test
    void testJavaScriptStacktraceParsing() {
        String stacktrace = "Error: Something failed\n"
                + "    at doSomething (/app/index.js:10:5)\n"
                + "    at main (/app/index.js:20:3)\n";

        List<CauseData.ExceptionData> exceptions = CauseTranslator.parseException(
                "Error", "Something failed",
                stacktrace, false, "javascript");

        assertThat(exceptions).hasSize(1);
        assertThat(exceptions.get(0).getStack()).hasSize(2);
        assertThat(exceptions.get(0).getStack().get(0).getPath()).isEqualTo("/app/index.js");
        assertThat(exceptions.get(0).getStack().get(0).getLabel()).isEqualTo("doSomething ");
        assertThat(exceptions.get(0).getStack().get(0).getLine()).isEqualTo(10);
    }

    @Test
    void testDotnetStacktraceParsing() {
        String stacktrace = "System.NullReferenceException: Object reference not set\n"
                + "   at MyApp.MyClass.MyMethod() in /src/MyClass.cs:line 42\n"
                + "   at MyApp.Program.Main(String[] args) in /src/Program.cs:55\n";

        List<CauseData.ExceptionData> exceptions = CauseTranslator.parseException(
                "System.NullReferenceException", "Object reference not set",
                stacktrace, false, "dotnet");

        assertThat(exceptions).hasSize(1);
        assertThat(exceptions.get(0).getStack()).hasSize(2);
        assertThat(exceptions.get(0).getStack().get(0).getPath()).isEqualTo("/src/MyClass.cs");
        assertThat(exceptions.get(0).getStack().get(0).getLabel()).isEqualTo("MyApp.MyClass.MyMethod()");
        assertThat(exceptions.get(0).getStack().get(0).getLine()).isEqualTo(42);
    }

    @Test
    void testGoStacktraceParsing() {
        String stacktrace = "goroutine 1 [running]:\n"
                + "main.funcA()\n"
                + "\t/home/user/project/main.go:10 +0x50\n"
                + "main.main()\n"
                + "\t/home/user/project/main.go:5 +0x20\n";

        List<CauseData.ExceptionData> exceptions = CauseTranslator.parseException(
                "panic", "runtime error",
                stacktrace, false, "go");

        assertThat(exceptions).hasSize(1);
        assertThat(exceptions.get(0).getStack()).hasSize(2);
        assertThat(exceptions.get(0).getStack().get(0).getPath()).isEqualTo("/home/user/project/main.go");
        assertThat(exceptions.get(0).getStack().get(0).getLabel()).isEqualTo("main.funcA()");
        assertThat(exceptions.get(0).getStack().get(0).getLine()).isEqualTo(10);
    }

    @Test
    void testEmptyStacktrace() {
        List<CauseData.ExceptionData> exceptions = CauseTranslator.parseException(
                "Error", "message", "", false, "java");

        assertThat(exceptions).hasSize(1);
        assertThat(exceptions.get(0).getStack()).isNull();
    }

    @Test
    void testUnknownLanguage() {
        List<CauseData.ExceptionData> exceptions = CauseTranslator.parseException(
                "Error", "message", "some stacktrace", false, "ruby");

        assertThat(exceptions).hasSize(1);
        assertThat(exceptions.get(0).getStack()).isNull();
    }
}
