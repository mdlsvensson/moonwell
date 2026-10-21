namespace Moonwell
{
    /// <summary>Converts the attack and defense types between Moonwell's numbers (World Editor's) and the game's, by name.</summary>
    internal static class TypeCodes
    {
        /// <summary>The game's number of an attack type; Unknown is spells, 0.</summary>
        internal static int AttackCode(AttackType type)
        {
            switch (type)
            {
                case AttackType.Normal:
                    return 1;
                case AttackType.Pierce:
                    return 2;
                case AttackType.Siege:
                    return 3;
                case AttackType.Magic:
                    return 4;
                case AttackType.Chaos:
                    return 5;
                case AttackType.Hero:
                    return 6;
                default:
                    return 0;
            }
        }

        /// <summary>The attack type of a game number; a number the game has none for reads as spells.</summary>
        internal static AttackType AttackOf(int code)
        {
            switch (code)
            {
                case 1:
                    return AttackType.Normal;
                case 2:
                    return AttackType.Pierce;
                case 3:
                    return AttackType.Siege;
                case 4:
                    return AttackType.Magic;
                case 5:
                    return AttackType.Chaos;
                case 6:
                    return AttackType.Hero;
                default:
                    return AttackType.Spells;
            }
        }

        /// <summary>The game's number of a defense type.</summary>
        internal static int DefenseCode(DefenseType type)
        {
            switch (type)
            {
                case DefenseType.Small:
                    return 0;
                case DefenseType.Medium:
                    return 1;
                case DefenseType.Large:
                    return 2;
                case DefenseType.Fort:
                    return 3;
                case DefenseType.Hero:
                    return 5;
                case DefenseType.Divine:
                    return 6;
                case DefenseType.None:
                    return 7;
                default:
                    return 4;
            }
        }

        /// <summary>The defense type of a game number; a number the game has none for reads as normal.</summary>
        internal static DefenseType DefenseOf(int code)
        {
            switch (code)
            {
                case 0:
                    return DefenseType.Small;
                case 1:
                    return DefenseType.Medium;
                case 2:
                    return DefenseType.Large;
                case 3:
                    return DefenseType.Fort;
                case 5:
                    return DefenseType.Hero;
                case 6:
                    return DefenseType.Divine;
                case 7:
                    return DefenseType.None;
                default:
                    return DefenseType.Normal;
            }
        }
    }
}
