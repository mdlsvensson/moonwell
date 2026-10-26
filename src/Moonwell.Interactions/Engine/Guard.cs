using System;
using System.Runtime.ExceptionServices;

namespace Moonwell.Engine
{
    /// <summary>
    /// Runs the reactions the cycle calls many times a step, and an interaction's check, guarded, without the closures a try
    /// makes in Lua. CSharp.lua compiles a try to System.try with a closure for the body and one for the catch, and a box for
    /// each local they capture, at every call. Here a call is one xpcall of the method itself with its arguments, under Turn,
    /// which turns an error as CoreSystem's own handler does. Each call gives the error the method raised, or null, and the
    /// caller reports it or raises it again as its catch or finally did. .NET runs the bodies: a try and catch.
    /// </summary>
    internal static class Guard
    {
        /// <summary>What the Accepts that Accept last ran answered; false when it threw.</summary>
        internal static bool answer;

        // Raw: xpcall of the method with its arguments, in place of System.try's closures. Safe: Turn is the handler, as
        // CoreSystem's is for a try, so the error is the same exception, its traceback taken before the stack unwinds; on
        // success OnTick returns nothing, so select gives null.
        /// <summary>Runs an interaction's OnTick for a pair, and gives the error it raised, or null.</summary>
        /// @CSharpLua.Template = "select(2, xpcall({0}.OnTick, Moonwell.Engine.Guard.Turn, {0}, {1}))"
        internal static Exception? Tick<H, O>(Interaction<H, O> interaction, Interaction<H, O>.Pair pair)
            where H : Actor
            where O : Actor
        {
            try
            {
                interaction.OnTick(pair);
                return null;
            }
            catch (Exception error)
            {
                return error;
            }
        }

        // Raw: as Tick, for OnMeet.
        /// <summary>Runs an interaction's OnMeet for a pair, and gives the error it raised, or null.</summary>
        /// @CSharpLua.Template = "select(2, xpcall({0}.OnMeet, Moonwell.Engine.Guard.Turn, {0}, {1}))"
        internal static Exception? Meet<H, O>(Interaction<H, O> interaction, Interaction<H, O>.Pair pair)
            where H : Actor
            where O : Actor
        {
            try
            {
                interaction.OnMeet(pair);
                return null;
            }
            catch (Exception error)
            {
                return error;
            }
        }

        // Raw: as Tick, for OnPart.
        /// <summary>Runs an interaction's OnPart for a pair, and gives the error it raised, or null.</summary>
        /// @CSharpLua.Template = "select(2, xpcall({0}.OnPart, Moonwell.Engine.Guard.Turn, {0}, {1}))"
        internal static Exception? Part<H, O>(Interaction<H, O> interaction, Interaction<H, O>.Pair pair)
            where H : Actor
            where O : Actor
        {
            try
            {
                interaction.OnPart(pair);
                return null;
            }
            catch (Exception error)
            {
                return error;
            }
        }

        // Raw: as Tick, for Accepts, whose answer Settle keeps. Safe: xpcall gives true and the answer, or false and the
        // error, and Settle takes both.
        /// <summary>Runs an interaction's Accepts for an other, keeps its answer in answer (false when it threw), and gives the error it raised, or null.</summary>
        /// @CSharpLua.Template = "Moonwell.Engine.Guard.Settle(xpcall({0}.Accepts, Moonwell.Engine.Guard.Turn, {0}, {1}))"
        internal static Exception? Accept<H, O>(Interaction<H, O> interaction, O other)
            where H : Actor
            where O : Actor
        {
            try
            {
                answer = interaction.Accepts(other);
                return null;
            }
            catch (Exception error)
            {
                answer = false;
                return error;
            }
        }

        // Raw: as Tick, for a gizmo's OnTick, which takes nothing.
        /// <summary>Runs a gizmo's OnTick, and gives the error it raised, or null.</summary>
        /// @CSharpLua.Template = "select(2, xpcall({0}.OnTick, Moonwell.Engine.Guard.Turn, {0}))"
        internal static Exception? Tick(Gizmo gizmo)
        {
            try
            {
                gizmo.OnTick();
                return null;
            }
            catch (Exception error)
            {
                return error;
            }
        }

        // Raw: as Tick, for the part of an interaction's check that its try covered; CheckNear returns nothing.
        /// <summary>Runs the part of an interaction's check after the holder's read, and gives the error it raised, or null.</summary>
        /// @CSharpLua.Template = "select(2, xpcall({0}.CheckNear, Moonwell.Engine.Guard.Turn, {0}))"
        internal static Exception? Check(Interaction interaction)
        {
            try
            {
                interaction.CheckNear();
                return null;
            }
            catch (Exception error)
            {
                return error;
            }
        }

        // Raw: error, as System.try raises again the error of a try with a finally. Safe: the error is the exception Turn
        // gave, whose traceback is taken, so nothing is lost.
        /// <summary>Raises again an error a guarded call gave, as a finally does after it has run.</summary>
        /// @CSharpLua.Template = "error({0})"
        internal static void Raise(Exception error)
        {
            ExceptionDispatchInfo.Capture(error).Throw();
        }

        /// <summary>Lua only: keeps what xpcall gave for Accepts, the answer on success, false otherwise, and gives the error, or null.</summary>
        internal static Exception? Settle(bool ok, object? result)
        {
            if (ok)
            {
                answer = Truth(result);
                return null;
            }

            answer = false;
            return Failure(result);
        }

        /// <summary>
        /// Lua only: the handler of every guarded call, the same as CoreSystem's for a try. No error becomes an Exception
        /// "script error"; the text of a Lua error becomes a NullReferenceException when it is an index of nil, a
        /// DivideByZeroException when it is a division by zero, and an Exception otherwise, each with its traceback taken here,
        /// before the stack unwinds; an exception the code threw is given as it is.
        /// </summary>
        internal static object Turn(object? error)
        {
            if (error == null)
            {
                var made = new Exception("script error");
                Trace(made);
                return made;
            }

            if (!IsText(error))
            {
                return error;
            }

            var text = Text(error);
            Exception turned;
            if (Has(text, "attempt to index"))
            {
                turned = new NullReferenceException(text);
            }
            else if (Has(text, "attempt to divide by zero"))
            {
                turned = new DivideByZeroException(text);
            }
            else
            {
                turned = new Exception(text);
            }

            Trace(turned);
            return turned;
        }

        // Raw: in Lua a C# bool is a Lua boolean, so the answer is the value xpcall gave. Safe: Accepts returns a bool.
        /// <summary>The answer Accepts gave, which xpcall passed on as an object.</summary>
        /// @CSharpLua.Template = "{0}"
        private static bool Truth(object? result)
        {
            return result is bool accepted && accepted;
        }

        // Raw: in Lua the error is its own table. Safe: Turn gave it, and Turn gives an exception for every error but one
        // the code threw, which is the exception it threw.
        /// <summary>The error xpcall passed on as an object.</summary>
        /// @CSharpLua.Template = "{0}"
        private static Exception? Failure(object? result)
        {
            return result as Exception;
        }

        // Raw: CoreSystem's handler asks type for a Lua error's text.
        /// <summary>Whether an error is a Lua error's text.</summary>
        /// @CSharpLua.Template = "(type({0}) == 'string')"
        private static bool IsText(object error)
        {
            return error is string;
        }

        // Raw: in Lua a text is a Lua string. Safe: IsText said it is one.
        /// <summary>An error that is a text, as the text.</summary>
        /// @CSharpLua.Template = "{0}"
        private static string Text(object error)
        {
            return (string)error;
        }

        // Raw: string.find, as CoreSystem's handler finds the kind of a Lua error. Safe: the parts have no pattern's signs.
        /// <summary>Whether a text holds a part.</summary>
        /// @CSharpLua.Template = "(string.find({0}, {1}) ~= nil)"
        private static bool Has(string text, string part)
        {
            return text.Contains(part);
        }

        // Raw: the traceback CoreSystem's handler takes of an exception it made, which moonwell.lua reads the C# lines from.
        /// <summary>Takes an exception's traceback, where the error was raised.</summary>
        /// @CSharpLua.Template = "{0}:traceback()"
        private static void Trace(Exception error)
        {
        }
    }
}
