namespace Moonwell
{
    /// <summary>Dealing damage from code: <c>source.Damage(target, 10f, DamageKind.Fire)</c>.</summary>
    public static class UnitDamage
    {
        /// <summary>Deals damage from a unit to another, through every stage up to lethal before it returns; landed and burst come 0 s later. Returns the hit, or null when the game dealt nothing. Dealt from a reaction, the hit continues that reaction's chain; a chain of 8 hits is cut there with an error.</summary>
        /// <param name="source">The unit that deals the damage.</param>
        /// <param name="target">The unit that takes it.</param>
        /// <param name="amount">The amount, before armor.</param>
        /// <param name="type">The damage type: magic by default, which spell immunity stops.</param>
        /// <param name="attackType">The attack type: spells by default, which is what the game gives a spell's damage.</param>
        /// <param name="isAttack">Whether reactions see the hit as an attack.</param>
        /// <param name="ranged">Whether an attack is ranged; reactions see it as melee otherwise.</param>
        /// <param name="sound">The sound of the hit.</param>
        /// <param name="tag">Anything of the map's own, which reactions read as Hit.Tag.</param>
        public static Hit? Damage(this Unit source, Unit target, float amount, DamageKind type = DamageKind.Magic, AttackType attackType = AttackType.Spells, bool isAttack = false, bool ranged = false, WeaponSound sound = WeaponSound.None, object? tag = null)
        {
            return Pipeline.Deal(source, target, amount, TypeCodes.AttackCode(attackType), type, isAttack, ranged, sound, tag);
        }
    }
}
