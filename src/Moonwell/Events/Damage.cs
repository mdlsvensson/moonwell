namespace Moonwell
{
    /// <summary>One damage dealt to a unit: its target, its source when known, its amount, and whether it is an attack. A damage package hands a class of its own that extends it.</summary>
    public class Damage
    {
        /// <summary>Makes the damage of one event.</summary>
        internal Damage(Unit target, Unit? source, float amount, bool isAttack)
        {
            Target = target;
            Source = source;
            Amount = amount;
            IsAttack = isAttack;
        }

        /// <summary>The unit that takes the damage.</summary>
        [LocalSafe]
        public Unit Target { get; }

        /// <summary>The unit that deals the damage; null when it is not known.</summary>
        [LocalSafe]
        public Unit? Source { get; }

        /// <summary>The amount of the damage; a reaction may change it.</summary>
        [LocalSafe]
        public float Amount { get; set; } = 0.0f;

        /// <summary>The tag the code that dealt the damage gave it (Unit.Damage); null otherwise.</summary>
        [LocalSafe]
        public object? Tag { get; internal set; }

        /// <summary>Whether the damage is an attack.</summary>
        [LocalSafe]
        public bool IsAttack { get; }
    }
}
