using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>
    /// Cases for C# that CSharp.lua wrote wrong until the fork's 2.0.4-moonwell.10: a lambda leaving a temporary as a
    /// statement (C28), a return with code after it (C29), a continue inside a switch (C30), a primary constructor's
    /// parameters (C25, C31), two object initializers in one statement and a foreach left early over an iterator (C25);
    /// and until 2.0.4-moonwell.11, a field named as a base class's private field (C32).
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
