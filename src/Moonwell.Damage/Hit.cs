namespace Moonwell
{
    /// <summary>One hit, as the damage package hands it to every stage: the damage with its types, where it came from, and what it did.</summary>
    public sealed class Hit : Damage
    {
        /// <summary>The damage kind, kept behind the property that checks the stage.</summary>
        private DamageKind damageKind;

        /// <summary>The weapon sound, kept behind the property that checks the stage.</summary>
        private WeaponSound weaponSound;

        /// <summary>The target's armor for this hit, kept behind the property that checks the stage.</summary>
        private float armor = 0.0f;

        /// <summary>Makes a hit that starts with the amount.</summary>
        internal Hit(Unit target, Unit? source, float amount, bool isAttack)
            : base(target, source, amount, isAttack)
        {
            Original = amount;
        }

        /// <summary>The amount the hit started with, before any reaction changed it.</summary>
        [LocalSafe]
        public float Original { get; } = 0.0f;

        /// <summary>Whether the hit was dealt by code, through Unit.Damage.</summary>
        [LocalSafe]
        public bool IsCode { get; internal set; }

        /// <summary>Whether the hit is a spell's: not an attack, with the attack type the game gives spells.</summary>
        [LocalSafe]
        public bool IsSpell => !IsAttack && AttackTypeCode == 0;

        /// <summary>Whether the hit is a melee attack.</summary>
        [LocalSafe]
        public bool IsMelee { get; internal set; }

        /// <summary>Whether the hit is a ranged attack.</summary>
        [LocalSafe]
        public bool IsRanged { get; internal set; }

        /// <summary>The damage kind, which the game calls its damage type; a reaction may change it before armor only.</summary>
        [LocalSafe]
        public DamageKind DamageKind
        {
            get => damageKind;
            set
            {
                CheckBeforeArmor("DamageKind");
                damageKind = value;
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

        /// <summary>The attack type; a reaction may change it before armor only. A spell's is <see cref="AttackType.Spells"/>.</summary>
        [LocalSafe]
        public AttackType AttackType
        {
            get => TypeCodes.AttackOf(AttackTypeCode);
            set
            {
                CheckBeforeArmor("AttackType");
                AttackTypeCode = TypeCodes.AttackCode(value);
            }
        }

        /// <summary>The target's defense type for this hit; a reaction may change it before armor only, and the target has its own defense type back after the hit.</summary>
        [LocalSafe]
        public DefenseType DefenseType
        {
            get => TypeCodes.DefenseOf(DefenseTypeCode);
            set
            {
                CheckBeforeArmor("DefenseType");
                DefenseTypeCode = TypeCodes.DefenseCode(value);
            }
        }

        /// <summary>Whether the hit killed its target; known once the hit has landed.</summary>
        [LocalSafe]
        public bool Killed { get; internal set; }

        /// <summary>The game's number of the attack type.</summary>
        internal int AttackTypeCode;

        /// <summary>The game's number of the target's defense type for this hit.</summary>
        internal int DefenseTypeCode;

        /// <summary>How many hits led to this one, each made while the reactions of the one before ran: 0 for a hit made while no reaction ran.</summary>
        internal int Chain;

        /// <summary>The stage the hit is in.</summary>
        internal Stage Stage;

        /// <summary>The target's life when the lethal check ran: after the after-armor reactions, so after any hit they dealt, and before this hit lowers it.</summary>
        internal float TargetLife = 0.0f;

        /// <summary>Whether the hit, at its amount now, leaves the target at death's life or below.</summary>
        internal bool Kills => TargetLife - Amount <= Pipeline.DeathLife;

        /// <summary>Whether the hit left its after-armor stage killing a living target; Killed is this and the target dead 0 s later.</summary>
        internal bool WouldKill;

        /// <summary>Whether no package reaction runs for the hit and it does not land: the game started it at 0, or it was cut.</summary>
        internal bool Quiet;

        /// <summary>Whether the hit was cut as the ninth of a chain: set to 0, and core's reactions do not run for it either.</summary>
        internal bool Cut;

        /// <summary>Whether the package put armor on the target for this hit, to be put back.</summary>
        internal bool ArmorChanged;

        /// <summary>The armor to set on the target to put its own back.</summary>
        internal float ArmorRestore = 0.0f;

        /// <summary>Whether the package put a defense type on the target for this hit, to be put back.</summary>
        internal bool DefenseChanged;

        /// <summary>The target's own defense type, to be put back.</summary>
        internal int DefenseRestore;

        /// <summary>In the lethal stage: lowers the amount so that the target keeps that much life.</summary>
        public void Spare(float life = 1.0f)
        {
            Dev.Check(Stage == Stage.Lethal, "Hit.Spare can be called only in the lethal stage (OnKilling, OnDying, OnUnitDying)");
            if (Stage != Stage.Lethal)
            {
                // A release build has no check: outside the lethal stage the life it would keep is not known.
                return;
            }

            var amount = TargetLife - life;
            Amount = amount < 0.0f ? 0.0f : amount;
        }

        /// <summary>Sets a type or the armor without the stage check; the package fills a new hit with it.</summary>
        internal void Fill(DamageKind type, WeaponSound sound, float targetArmor)
        {
            damageKind = type;
            weaponSound = sound;
            armor = targetArmor;
        }

        /// <summary>Reports a change made after armor, which the game would ignore.</summary>
        private void CheckBeforeArmor(string member)
        {
            Dev.Check(Stage == Stage.BeforeArmor, "Hit." + member + " can be changed only before armor (OnDealing, OnTaking, OnUnitTaking)");
        }
    }
}
