namespace Moonwell
{
    /// <summary>One hit, as the damage package hands it to every stage: the damage with its types, where it came from, and what it did.</summary>
    public sealed class Hit : Damage
    {
        /// <summary>The damage type, kept behind the property that checks the stage.</summary>
        private DamageType damageType;

        /// <summary>The weapon sound, kept behind the property that checks the stage.</summary>
        private WeaponSound weaponSound;

        /// <summary>The target's armor for this hit, kept behind the property that checks the stage.</summary>
        private float armor;

        /// <summary>Makes a hit that starts with the amount.</summary>
        internal Hit(Unit target, Unit? source, float amount, bool isAttack)
            : base(target, source, amount, isAttack)
        {
            Original = amount;
        }

        /// <summary>The amount the hit started with, before any reaction changed it.</summary>
        [LocalSafe]
        public float Original { get; }

        /// <summary>Whether the hit was dealt by code, through Unit.Damage.</summary>
        [LocalSafe]
        public bool IsCode { get; internal set; }

        /// <summary>The tag the code that dealt the hit gave it; null otherwise.</summary>
        [LocalSafe]
        public object? Tag { get; internal set; }

        /// <summary>Whether the hit is a spell's: not an attack, with the attack type the game gives spells.</summary>
        [LocalSafe]
        public bool IsSpell => !IsAttack && AttackTypeCode == 0;

        /// <summary>Whether the hit is a melee attack.</summary>
        [LocalSafe]
        public bool IsMelee { get; internal set; }

        /// <summary>Whether the hit is a ranged attack.</summary>
        [LocalSafe]
        public bool IsRanged { get; internal set; }

        /// <summary>The damage type; a reaction may change it before armor only.</summary>
        [LocalSafe]
        public DamageType DamageType
        {
            get => damageType;
            set
            {
                CheckBeforeArmor("DamageType");
                damageType = value;
            }
        }

        /// <summary>The sound of the hit, which the game calls its weapon type; a reaction may change it before armor only.</summary>
        [LocalSafe]
        public WeaponSound WeaponSound
        {
            get => weaponSound;
            set
            {
                CheckBeforeArmor("WeaponSound");
                weaponSound = value;
            }
        }

        /// <summary>The target's armor for this hit; a reaction may change it before armor only, and the target has its own armor back after the hit.</summary>
        [LocalSafe]
        public float Armor
        {
            get => armor;
            set
            {
                CheckBeforeArmor("Armor");
                armor = value;
            }
        }

        /// <summary>Whether the hit killed its target; known once the hit has landed.</summary>
        [LocalSafe]
        public bool Killed { get; internal set; }

        /// <summary>The game's number of the attack type.</summary>
        internal int AttackTypeCode;

        /// <summary>The game's number of the target's defense type for this hit.</summary>
        internal int DefenseTypeCode;

        /// <summary>How many hits dealt from reactions led to this one: 0 for a hit the game or plain code dealt.</summary>
        internal int Chain;

        /// <summary>The stage the hit is in.</summary>
        internal Stage Stage;

        /// <summary>The target's life when the after-armor stage began, before the hit lowered it.</summary>
        internal float LifeBefore;

        /// <summary>Whether the package put armor on the target for this hit, to be put back.</summary>
        internal bool ArmorChanged;

        /// <summary>The armor to set on the target to put its own back.</summary>
        internal float ArmorRestore;

        /// <summary>Whether the package put a defense type on the target for this hit, to be put back.</summary>
        internal bool DefenseChanged;

        /// <summary>The target's own defense type, to be put back.</summary>
        internal int DefenseRestore;

        /// <summary>In the lethal stage: lowers the amount so that the target keeps that much life.</summary>
        public void Spare(float life = 1f)
        {
            Dev.Check(Stage == Stage.Lethal, "Hit.Spare can be called only in the lethal stage (OnKilling, OnLethal)");
            var amount = LifeBefore - life;
            Amount = amount < 0f ? 0f : amount;
        }

        /// <summary>Sets a type or the armor without the stage check; the package fills a new hit with it.</summary>
        internal void Fill(DamageType type, WeaponSound sound, float targetArmor)
        {
            damageType = type;
            weaponSound = sound;
            armor = targetArmor;
        }

        /// <summary>Reports a change made after armor, which the game would ignore.</summary>
        private void CheckBeforeArmor(string member)
        {
            Dev.Check(Stage == Stage.BeforeArmor, "Hit." + member + " can be changed only before armor (OnDealing, OnTaking, OnBeforeArmor)");
        }
    }
}
