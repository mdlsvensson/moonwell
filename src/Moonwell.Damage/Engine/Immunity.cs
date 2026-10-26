namespace Moonwell
{
    /// <summary>Which hits spell immunity stops, as the damage probe of 2026-10-20 saw the game do it on 3.0.1.</summary>
    internal static class Immunity
    {
        /// <summary>The game's number of the magic attack type, which spell immunity stops with any damage type.</summary>
        private const int MagicAttack = 4;

        /// <summary>The damage types spell immunity stops with any attack type.</summary>
        private static readonly DamageKind[] stopped =
        {
            DamageKind.Fire, DamageKind.Cold, DamageKind.Lightning, DamageKind.Divine, DamageKind.Magic, DamageKind.Sonic,
            DamageKind.Force, DamageKind.Death, DamageKind.Mind, DamageKind.Plant, DamageKind.Defensive, DamageKind.SpiritLink,
            DamageKind.ShadowStrike,
        };

        /// <summary>Whether the game will skip a hit's after-armor event: the target is magic immune, and the attack type is magic or the damage type is one immunity stops.</summary>
        internal static bool Stops(bool magicImmune, int attackType, DamageKind damageType)
        {
            if (!magicImmune)
            {
                return false;
            }

            if (attackType == MagicAttack)
            {
                return true;
            }

            for (var i = 0; i < stopped.Length; i++)
            {
                if (stopped[i] == damageType)
                {
                    return true;
                }
            }

            return false;
        }
    }
}
