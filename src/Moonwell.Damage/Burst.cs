using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>Every hit one source landed in one moment, such as an attack's splash or a cleave.</summary>
    public sealed class Burst
    {
        /// <summary>Makes an empty burst of a source.</summary>
        internal Burst(Unit source)
        {
            Source = source;
        }

        /// <summary>The unit that dealt the hits.</summary>
        [LocalSafe]
        public Unit Source { get; }

        /// <summary>The hits, in the order they came.</summary>
        [LocalSafe]
        public List<Hit> Hits { get; } = new List<Hit>();

        /// <summary>How many units the hits hit.</summary>
        [LocalSafe]
        public int Targets { get; internal set; }
    }
}
