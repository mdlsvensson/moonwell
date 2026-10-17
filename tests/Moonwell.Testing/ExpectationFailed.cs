using System;

namespace Moonwell.Testing
{
    /// <summary>The exception a failed Expect check throws.</summary>
    public sealed class ExpectationFailed : Exception
    {
        /// <summary>Makes the failure with its message.</summary>
        public ExpectationFailed(string message)
            : base(message)
        {
        }
    }
}
