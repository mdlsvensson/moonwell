using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>
    /// Cases for C# that CSharp.lua wrote wrong until the fork's 2.0.4-moonwell.10: a lambda leaving a temporary as a
    /// statement (C28), a return with code after it (C29), a continue inside a switch (C30), a primary constructor's
    /// parameters (C25, C31), two object initializers in one statement and a foreach left early over an iterator (C25);
    /// until 2.0.4-moonwell.11, a field named as a base class's private field (C32); and until 2.0.4-moonwell.12, a local
    /// function used above its declaration (C36), a return after a using declaration (C37), a nullable struct compared
    /// with null (C38), patterns in a switch statement (C39) and the negation of a negative constant (C40).
    /// The build's line marks put a comment after a loop body's last return, which is the shape C29 was found in.
    /// </summary>
    public static class CompilerCases
    {
        /// <summary>A lambda that returns nothing, with an expression body calling a method with an out argument.</summary>
        public static void OutDiscardLambda(FakeWorld world)
        {
            var calls = new List<string>();
            Action discard = () => Stand(calls, out _);
            discard();
            Action<int> declared = n => Stand(calls, out var kept);
            declared(1);
            Action created = () => new Holder { Count = 2 };
            created();
            Expect.Equal(2, calls.Count, "both lambdas ran");
        }

        /// <summary>A loop with a continue whose last statement is a return, and a for loop written as a while.</summary>
        public static void ReturnLastInLoop(FakeWorld world)
        {
            var items = new List<int> { 0, 0, 3, 4 };
            Expect.Equal(3, FirstNonZero(items), "a foreach with a continue");
            Expect.Equal(2, FirstEven(9), "a for loop with a step");
        }

        /// <summary>A continue inside a switch skips the rest of the loop body.</summary>
        public static void ContinueInSwitch(FakeWorld world)
        {
            var seen = "";
            for (var n = 1; n <= 3; n++)
            {
                switch (n)
                {
                    case 2:
                        continue;
                }

                seen += n;
            }

            Expect.Equal("13", seen, "2 is skipped");
        }

        /// <summary>A primary constructor's parameter read by an initializer and passed to a base class.</summary>
        public static void PrimaryConstructors(FakeWorld world)
        {
            Expect.Equal(5, new Initialized(5).Value, "a field initializer");
            var derived = new NamedDerived("q");
            Expect.Equal("q!", derived.FromBase(), "the base class's parameter");
            Expect.Equal("q", derived.FromDerived(), "the derived class's parameter of the same name");
        }

        /// <summary>A derived class's field and its base class's private field of the same name are two fields.</summary>
        public static void PrivateBaseFields(FakeWorld world)
        {
            var counted = new CountedDerived();
            counted.Add();
            Expect.Equal(1, counted.Count, "the base class's count");
            Expect.Equal(100, counted.Own, "the derived class's count");
        }

        /// <summary>Two object initializers read in one statement keep their own objects.</summary>
        public static void InitializerTemporaries(FakeWorld world)
        {
            Expect.Equal("1 2", new Holder { Count = 1 }.Count + " " + new Holder { Count = 2 }.Count, "two initializers");
        }

        /// <summary>A foreach left by a break or a return runs the iterator's finally.</summary>
        public static void ForeachDisposes(FakeWorld world)
        {
            var log = new List<string>();
            foreach (var n in Numbers(log))
            {
                break;
            }

            Expect.Equal(1, log.Count, "a break");
            Expect.Equal(2, FirstOf(log), "the value a return gives");
            Expect.Equal(2, log.Count, "a return");
        }

        /// <summary>Local functions called above their declaration: plain, recursive, two that call each other, one with a local.</summary>
        public static void LocalFunctionBeforeDeclaration(FakeWorld world)
        {
            var scale = 3;
            Expect.Equal(5, Add(2, 3), "a call above the declaration");
            Expect.Equal(120, Factorial(5), "a recursive one");
            Expect.Equal(true, IsEven(10), "two that call each other");
            Expect.Equal(6, Scale(2), "one that reads a local declared above the call");

            int Add(int a, int b) => a + b;

            int Factorial(int n) => n <= 1 ? 1 : n * Factorial(n - 1);

            bool IsEven(int n) => n == 0 || IsOdd(n - 1);

            bool IsOdd(int n) => n != 0 && IsEven(n - 1);

            int Scale(int value) => value * scale;
        }

        /// <summary>A return inside an if, a loop or a switch after a using declaration gives its value.</summary>
        public static void UsingDeclarationReturns(FakeWorld world)
        {
            var log = new List<string>();
            Expect.Equal(1, ReturnInIf(log, true), "a return inside an if");
            Expect.Equal(2, ReturnInIf(log, false), "the return after it");
            Expect.Equal(5, ReturnInLoop(log, new List<int> { 1, 5, 7 }), "a return inside a loop");
            Expect.Equal("one", ReturnInSwitch(log, 1), "a return inside a switch");
            Expect.Equal(4, log.Count, "each one disposed");
        }

        /// <summary>A nullable struct with its own == compared with null, on either side.</summary>
        public static void NullableEqualsNull(FakeWorld world)
        {
            Vector2? none = null;
            Vector2? some = new Vector2(1f, 2f);
            Expect.Equal(true, none == null, "null == null");
            Expect.Equal(false, none != null, "null != null");
            Expect.Equal(false, some == null, "a value == null");
            Expect.Equal(true, null != some, "null != a value");
            Expect.Equal(true, some == new Vector2(1f, 2f), "a value == the same value");
        }

        /// <summary>Relational, not, parenthesized and property patterns in a switch statement's case labels.</summary>
        public static void SwitchStatementPatterns(FakeWorld world)
        {
            Expect.Equal("neg small big zero", Size(-3) + " " + Size(3) + " " + Size(9) + " " + Size(0), "relational patterns");
            Expect.Equal("low out mid", Range(2) + " " + Range(12) + " " + Range(5), "or, not and parentheses");
            Expect.Equal("nine", Inner(new Holder { Next = new Holder { Count = 9 } }), "an extended property pattern");
            Expect.Equal("not three", Inner(new Holder { Count = 4 }), "not inside a property pattern");
            Expect.Equal("none", Inner(null), "a property pattern does not match null");
        }

        /// <summary>A minus before a negative constant.</summary>
        public static void NegatedNegativeConstant(FakeWorld world)
        {
            var single = -NegativeSingle;
            var integer = -NegativeInteger;
            Expect.Near(2f, single, "a float constant");
            Expect.Equal(5, integer, "an int constant");
        }

        private const float NegativeSingle = -2f;

        private const int NegativeInteger = -5;

        private static int ReturnInIf(List<string> log, bool early)
        {
            using var logged = new Logged(log);
            if (early)
            {
                return 1;
            }

            return 2;
        }

        private static int ReturnInLoop(List<string> log, List<int> items)
        {
            using var logged = new Logged(log);
            foreach (var item in items)
            {
                if (item > 1)
                {
                    return item;
                }
            }

            return -1;
        }

        private static string ReturnInSwitch(List<string> log, int n)
        {
            using var logged = new Logged(log);
            switch (n)
            {
                case 1:
                    return "one";
                default:
                    return "other";
            }
        }

        private static string Size(int n)
        {
            switch (n)
            {
                case < 0:
                    return "neg";
                case >= 1 and <= 5:
                    return "small";
                case > 5:
                    return "big";
                default:
                    return "zero";
            }
        }

        private static string Range(int n)
        {
            switch (n)
            {
                case (1 or 2):
                    return "low";
                case not (> 2 and < 9):
                    return "out";
                default:
                    return "mid";
            }
        }

        private static string Inner(Holder? holder)
        {
            switch (holder)
            {
                case { Next.Count: 9 }:
                    return "nine";
                case { Count: not 3 }:
                    return "not three";
                default:
                    return "none";
            }
        }

        private static bool Stand(List<string> calls, out int nearest)
        {
            calls.Add("stand");
            nearest = 0;
            return true;
        }

        private static int FirstNonZero(List<int> items)
        {
            foreach (var item in items)
            {
                if (item == 0)
                {
                    continue;
                }

                return item; // a comment here, as the build's line marks put one
            }

            return -1;
        }

        private static int FirstEven(int limit)
        {
            for (var n = 1; n < limit && limit > 0; n += 1)
            {
                if (n % 2 == 1)
                {
                    continue;
                }

                return n;
            }

            return -1;
        }

        private static IEnumerable<int> Numbers(List<string> log)
        {
            try
            {
                yield return 2;
                yield return 3;
            }
            finally
            {
                log.Add("finally");
            }
        }

        private static int FirstOf(List<string> log)
        {
            foreach (var n in Numbers(log))
            {
                return n;
            }

            return -1;
        }

        private sealed class Holder
        {
            public int Count;

            public Holder? Next;
        }

        private sealed class Logged(List<string> log) : IDisposable
        {
            public void Dispose()
            {
                log.Add("disposed");
            }
        }

        private sealed class Initialized(int value)
        {
            public int Value = value;
        }

        private class NamedBase(string name)
        {
            public string FromBase() => name;
        }

        private sealed class NamedDerived(string name) : NamedBase(name + "!")
        {
            public string FromDerived() => name;
        }

        private class CountedBase
        {
            private int count;

            public int Count => count;

            public void Add()
            {
                count++;
            }
        }

        private sealed class CountedDerived : CountedBase
        {
            private int count = 100;

            public int Own => count;
        }
    }
}
