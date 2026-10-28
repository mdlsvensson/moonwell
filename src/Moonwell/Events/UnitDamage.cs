namespace Moonwell
{
    /// <summary>
    /// Dealing damage from code: <c>source.Damage(target, 10f, DamageKind.Fire)</c>. An extension, so a unit class whose
    /// declared field is Damage (Attack 1's base damage) still has it.
    /// </summary>
    public static class UnitDamage
    {
        /// <summary>
        /// Deals damage from a unit to another, as a spell does unless told otherwise: magic, which spell immunity
        /// stops, of the spells attack type, with no sound. The reactions run inside the call, and it returns the damage
        /// they saw, or one with the amount given when none ran; null when the game dealt nothing. With the
        /// Moonwell.Damage package started, the hit goes through its stages and is the package's Hit.
        /// </summary>
        /// <param name="source">The unit that deals the damage.</param>
        /// <param name="target">The unit that takes the damage.</param>
        /// <param name="amount">The amount, before armor.</param>
        /// <param name="kind">The damage type: magic by default.</param>
        /// <param name="attackType">The attack type: spells by default, what the game gives a spell's damage.</param>
        /// <param name="isAttack">Whether reactions see the damage as an attack.</param>
        /// <param name="ranged">Whether an attack is ranged; melee otherwise.</param>
        /// <param name="sound">The sound of the hit; none by default.</param>
        /// <param name="tag">Anything of the map's own, which reactions read as the damage's Tag.</param>
        public static Damage? Damage(this Unit source, Unit target, float amount, DamageKind kind = DamageKind.Magic, AttackType attackType = AttackType.Spells, bool isAttack = false, bool ranged = false, WeaponSound sound = WeaponSound.None, object? tag = null)
        {
            if (Events.DamageDealer != null)
            {
                return Events.DamageDealer(source, target, amount, kind, attackType, isAttack, ranged, sound, tag);
            }

            return Inbound.Deal(source, target, amount, kind, attackType, isAttack, ranged, sound, tag);
        }

        /// <summary>
        /// Deals damage from a unit to a destructable, as the game does for a spell: through the destructable's armor
        /// and the attack type. It returns nothing, as the damage object and the Moonwell.Damage package's stages are
        /// for units. A destructable that has ended takes none.
        /// </summary>
        /// <param name="source">The unit that deals the damage.</param>
        /// <param name="target">The destructable that takes the damage.</param>
        /// <param name="amount">The amount, before armor.</param>
        /// <param name="kind">The damage type: magic by default.</param>
        /// <param name="attackType">The attack type: spells by default, what the game gives a spell's damage.</param>
        /// <param name="isAttack">Whether the game sees the damage as an attack.</param>
        /// <param name="ranged">Whether an attack is ranged; melee otherwise.</param>
        /// <param name="sound">The sound of the hit; none by default.</param>
        public static void Damage(this Unit source, Destructable target, float amount, DamageKind kind = DamageKind.Magic, AttackType attackType = AttackType.Spells, bool isAttack = false, bool ranged = false, WeaponSound sound = WeaponSound.None)
        {
            if (target.HasEnded || target.IsGone)
            {
                return;
            }

            Runtime.World.DealDamageTo(source.Handle, target.Handle, amount, isAttack, ranged, TypeCodes.AttackCode(attackType), (int)kind, (int)sound);
        }
    }
}
