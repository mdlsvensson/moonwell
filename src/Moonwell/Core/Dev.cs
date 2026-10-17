using System.Diagnostics;

namespace Moonwell
{
    /// <summary>Checks that a development build makes while it runs; a release build leaves them out.</summary>
    internal static class Dev
    {
        /// <summary>Throws a MoonwellException with the message when the condition is false.</summary>
        [Conditional("DEBUG")]
        internal static void Check(bool condition, string message)
        {
            if (!condition)
            {
                throw new MoonwellException(message);
            }
        }
    }
}
