using System;

namespace Moonwell.Testing
{
    /// <summary>Checks a condition inside a case; a failed check throws ExpectationFailed.</summary>
    public static class Expect
    {
        /// <summary>Fails unless the two ints are equal.</summary>
        public static void Equal(int expected, int actual, string what)
        {
            if (expected != actual)
            {
                throw Mismatch(what, expected, actual);
            }
        }

        /// <summary>Fails unless the two strings are equal, where null equals null.</summary>
        public static void Equal(string? expected, string? actual, string what)
        {
            if (expected != actual)
            {
                throw Mismatch(what, expected, actual);
            }
        }

        /// <summary>Fails unless the two bools are equal.</summary>
        public static void Equal(bool expected, bool actual, string what)
        {
            if (expected != actual)
            {
                throw Mismatch(what, expected, actual);
            }
        }

        /// <summary>Fails unless both are the same object.</summary>
        public static void Same(object? expected, object? actual, string what)
        {
            if (!object.ReferenceEquals(expected, actual))
            {
                throw Mismatch(what, expected, actual);
            }
        }

        /// <summary>Fails unless the two floats are within 0.001; in the game's own Lua, whose square roots and trig are off by up to about 2 parts in 100,000, within 0.001 and 2 parts in 10,000 of the expected value.</summary>
        public static void Near(float expected, float actual, string what)
        {
            var tolerance = GameNumbers.HasNaN ? 0.001f : 0.001f + Math.Abs(expected) * 0.0002f;
            if (Math.Abs(expected - actual) > tolerance)
            {
                throw Mismatch(what, expected, actual);
            }
        }

        /// <summary>Fails unless the condition holds.</summary>
        public static void True(bool condition, string what)
        {
            if (!condition)
            {
                throw new ExpectationFailed(what);
            }
        }

        /// <summary>Fails unless the action throws a TException.</summary>
        public static void Throws<TException>(Action action, string what)
            where TException : Exception
        {
            try
            {
                action();
            }
            catch (Exception error)
            {
                if (error is TException)
                {
                    return;
                }

                throw new ExpectationFailed(what + ": threw " + error.Message);
            }

            throw new ExpectationFailed(what + ": nothing was thrown");
        }

        private static ExpectationFailed Mismatch(string what, object? expected, object? actual)
        {
            return new ExpectationFailed(what + ": expected " + expected + ", got " + actual);
        }
    }
}
