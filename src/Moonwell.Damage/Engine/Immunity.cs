namespace Moonwell
{
    /// <summary>Which hits spell immunity stops, as the damage probe of 2026-10-20 saw the game do it on 3.0.1.</summary>
    internal static class Immunity
    {
        /// <summary>The damage types spell immunity stops with any attack type: fire, cold, lightning, divine, magic, sonic, force, death, mind, plant, defensive, spirit link, shadow strike.</summary>
        private static readonly int[] stopped = { 8, 9, 10, 13, 14, 15, 17, 18, 19, 20, 21, 24, 25 };

        /// <summary>Whether the game will skip a hit's after-armor event: the target is magic immune, and the attack type is magic (4) or the damage type is one immunity stops.</summary>
        internal static bool Stops(bool magicImmune, int attackType, int damageType)
        {
            if (!magicImmune)
            {
                return false;
            }

            if (attackType == 4)
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
