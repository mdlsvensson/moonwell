using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A game component that keeps every unit OnUnitCreated is given, in turn.</summary>
    public sealed class CreatedWatch : GameComponent
    {
        /// <summary>The units OnUnitCreated was given.</summary>
        public readonly List<Unit> Seen = new List<Unit>();

        /// <summary>Keeps the unit.</summary>
        public override void OnUnitCreated(Unit createdUnit)
        {
            Seen.Add(createdUnit);
        }
    }
}
