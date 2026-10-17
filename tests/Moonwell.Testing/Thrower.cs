using System;

namespace Moonwell.Testing
{
    /// <summary>A game component whose OnAdded throws, for the actor cases.</summary>
    public sealed class Thrower : GameComponent
    {
        /// <summary>Throws an InvalidOperationException with the message boom.</summary>
        protected internal override void OnAdded()
        {
            throw new InvalidOperationException("boom");
        }
    }
}
