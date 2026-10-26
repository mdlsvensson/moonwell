using System;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for the handler the package's guarded calls run under, which in Lua must turn an error as CoreSystem's own handler for a try does.</summary>
    public static class GuardCases
    {
        // In Lua moonwell.lua's traceback keeps where an exception was raised in moonwellWhere, which is nil until it runs.
        /// <summary>Whether an exception's traceback was taken; always in .NET, which has none to take.</summary>
        /// @CSharpLua.Template = "({0}.moonwellWhere ~= nil)"
        private static bool Traced(Exception error)
        {
            return true;
        }

        /// <summary>No error becomes an Exception "script error"; a Lua error's text becomes a NullReferenceException for an index of nil, a DivideByZeroException for a division by zero and an Exception otherwise, each with the text and its traceback taken; an exception the code threw comes back as it is.</summary>
        public static void Turn(FakeWorld world)
        {
            var none = Guard.Turn(null) as Exception;
            Expect.True(none != null, "no error becomes an exception");
            Expect.Equal("script error", none!.Message, "with the text CoreSystem gives it");
            Expect.True(!(none is NullReferenceException) && !(none is DivideByZeroException), "a plain Exception");
            Expect.True(Traced(none), "its traceback is taken");
            var index = Guard.Turn("x.lua:3: attempt to index a nil value (local 'u')") as Exception;
            Expect.True(index is NullReferenceException, "an index of nil is a NullReferenceException");
            Expect.Equal("x.lua:3: attempt to index a nil value (local 'u')", index!.Message, "with the error's text");
            Expect.True(Traced(index), "and its traceback taken");
            var divide = Guard.Turn("attempt to divide by zero") as Exception;
            Expect.True(divide is DivideByZeroException, "a division by zero is a DivideByZeroException");
            Expect.True(Traced(divide!), "with its traceback");
            var other = Guard.Turn("bad argument #1 to 'floor'") as Exception;
            Expect.True(other != null && !(other is NullReferenceException) && !(other is DivideByZeroException), "any other text is a plain Exception");
            Expect.Equal("bad argument #1 to 'floor'", other!.Message, "with the text");
            Expect.True(Traced(other), "and its traceback");
            var thrown = new InvalidOperationException("boom");
            Expect.Same(thrown, Guard.Turn(thrown), "an exception the code threw comes back as it is");
        }
    }
}
