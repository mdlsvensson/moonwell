namespace Moonwell
{
    /// <summary>One cast of a spell: it begins when the spell takes effect, and ends at the spell's end or when the caster's next cast begins.</summary>
    [LocalSafe]
    public sealed class Cast : Scope
    {
        /// <summary>Makes the cast of an ability by a caster, at a target and a place, with the level the ability has now.</summary>
        internal Cast(Ability ability, Unit caster, Unit? target, float x, float y, int level)
        {
            Ability = ability;
            Caster = caster;
            Target = target;
            X = x;
            Y = y;
            Level = level;
        }

        /// <summary>The ability being cast.</summary>
        public Ability Ability { get; }

        /// <summary>The unit casting the ability.</summary>
        public Unit Caster { get; }

        /// <summary>The unit the spell is cast on; null when there is none.</summary>
        public Unit? Target { get; }

        /// <summary>The x of the place the spell is cast at.</summary>
        public float X { get; } = 0.0f;

        /// <summary>The y of the place the spell is cast at.</summary>
        public float Y { get; } = 0.0f;

        /// <summary>The level of the ability when the cast began.</summary>
        public int Level { get; }
    }
}
