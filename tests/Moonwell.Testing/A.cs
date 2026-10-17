using System;

namespace Moonwell.Testing
{
    /// <summary>A unit component whose OnDamaged throws, for the error case.</summary>
    public class A : UnitComponent
    {
        /// <summary>Throws an InvalidOperationException with the message boom.</summary>
        public override void OnDamaged(Damage damage)
        {
            throw new InvalidOperationException("boom");
        }
    }
}
