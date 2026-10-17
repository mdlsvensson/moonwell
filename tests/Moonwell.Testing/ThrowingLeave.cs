using System;

namespace Moonwell.Testing
{
    /// <summary>A unit whose OnLeave throws, for the sweep cases.</summary>
    public class ThrowingLeave : Unit
    {
        /// <summary>Throws an InvalidOperationException with the message boom.</summary>
        protected override void OnLeave()
        {
            throw new InvalidOperationException("boom");
        }
    }
}
