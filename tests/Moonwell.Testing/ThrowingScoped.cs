using System;

namespace Moonwell.Testing
{
    /// <summary>An owned object whose end throws, for the ownership cases.</summary>
    public class ThrowingScoped : Scoped
    {
        /// <summary>Throws an InvalidOperationException with the message boom.</summary>
        protected override void OnEnd()
        {
            throw new InvalidOperationException("boom");
        }
    }
}
