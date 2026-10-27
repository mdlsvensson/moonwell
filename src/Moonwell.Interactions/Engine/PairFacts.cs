#if DEBUG
namespace Moonwell.Engine
{
    /// <summary>What the debug mode reads of a pair. A pair is kept as an Owner, and only its typed interaction can read it (see Pairing), so Interaction.ReadPair fills one of these, which the view reuses.</summary>
    internal sealed class PairFacts
    {
        /// <summary>The holder of the pair's interaction.</summary>
        public Actor Holder = null!;

        /// <summary>The thing the holder met.</summary>
        public Actor Other = null!;

        /// <summary>The distance between the two at the last check.</summary>
        public float Distance;

        /// <summary>The holder's place at the last check, left to right.</summary>
        public float HolderX;

        /// <summary>The holder's place at the last check, bottom to top.</summary>
        public float HolderY;

        /// <summary>The other's place at the last check, left to right.</summary>
        public float OtherX;

        /// <summary>The other's place at the last check, bottom to top.</summary>
        public float OtherY;

        /// <summary>The seconds since the two met.</summary>
        public float Since;

        /// <summary>The seconds left of a Wait; 0 when none runs.</summary>
        public float Waits;

        /// <summary>The seconds left of a cooldown; 0 when none runs.</summary>
        public float Cooldown;

        /// <summary>Whether the pair is paused.</summary>
        public bool Paused;

        /// <summary>How many times its OnTick has run.</summary>
        public int Ticked;
    }
}
#endif
