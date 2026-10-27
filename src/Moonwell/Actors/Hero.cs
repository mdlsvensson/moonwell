namespace Moonwell
{
    /// <summary>A hero unit: the class of a unit the game makes as a hero when no class is registered for its type. It has experience, a level, stats and skill points, and can be revived.</summary>
    public class Hero : Unit
    {
        // The instance fields are internal, not private, as a map's subclass may name a field of its own the same (see Owner).

        /// <summary>The experience last read or set.</summary>
        internal int lastExperience;

        /// <summary>The base strength last read or set.</summary>
        internal int lastStrength;

        /// <summary>The base agility last read or set.</summary>
        internal int lastAgility;

        /// <summary>The base intelligence last read or set.</summary>
        internal int lastIntelligence;

        /// <summary>The strength with bonuses last read.</summary>
        internal int lastStrengthWithBonus;

        /// <summary>The agility with bonuses last read.</summary>
        internal int lastAgilityWithBonus;

        /// <summary>The intelligence with bonuses last read.</summary>
        internal int lastIntelligenceWithBonus;

        /// <summary>The skill points last read or set.</summary>
        internal int lastSkillPoints;

        /// <summary>The proper name last set.</summary>
        internal string lastProperName = "";

        /// <summary>The default object of the hero's type (see Unit.Default), a Hero.</summary>
        [LocalSafe]
        public new Hero Default => (Hero)base.Default;

        /// <summary>The hero's proper name (GetHeroProperName), shown above its type's name. The game gives it in each player's language, so it is read only inside Local.For. Setting it (BlzSetHeroProperName) changes only the proper name, the same on every machine. Once the game has removed the hero, the last value set (a read keeps nothing, as it differs by machine), and setting does nothing.</summary>
        public string ProperName
        {
            [MachineLocal]
            get
            {
                if (IsGone)
                {
                    return lastProperName;
                }

                var now = Runtime.World.ProperNameOf(Handle);
                if (string.IsNullOrEmpty(now) && RemovedByGame())
                {
                    return lastProperName;
                }

                return now;
            }

            set
            {
                if (!IsGone)
                {
                    lastProperName = value;
                    Runtime.World.SetProperName(Handle, value);
                }
            }
        }

        /// <summary>The hero's experience. Setting a higher value makes the game give the levels that come with it, with the level-up effect (SetHeroXP, which only raises; it stops at the last level's experience). Setting a lower value lowers the experience but keeps the level (AddHeroXP with a negative amount); SetLevel lowers the level. A read after a set gives what the game took. Once the game has removed the hero, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public int Experience
        {
            get
            {
                if (IsGone)
                {
                    return lastExperience;
                }

                var now = Runtime.World.ExperienceOf(Handle);
                if (now == 0 && RemovedByGame())
                {
                    return lastExperience;
                }

                if (Local.Depth == 0)
                {
                    lastExperience = now;
                }

                return now;
            }

            set
            {
                if (!IsGone)
                {
                    // SetHeroXP only raises (seen 10-21); a negative AddHeroXP lowers the experience and keeps the level.
                    var now = Runtime.World.ExperienceOf(Handle);
                    if (value < now)
                    {
                        Runtime.World.AddExperience(Handle, value - now, true);
                    }
                    else
                    {
                        Runtime.World.SetExperience(Handle, value, true);
                    }

                    // The last value is what the game took, which the getter keeps.
                    lastExperience = Experience;
                }
            }
        }

        /// <summary>Gives the hero a level, as blizzard.j's SetHeroLevelBJ does: a higher one with the level-up effect unless showEffect is false (SetHeroLevel, which only raises, and gives the hero that level's experience; it stops at the last level), a lower one by taking the levels away (UnitStripHeroLevel). Level and Experience read what the game took. Does nothing once the hero is gone.</summary>
        public void SetLevel(int level, bool showEffect = true)
        {
            if (!IsGone)
            {
                var now = Runtime.World.LevelOf(Handle);
                if (level > now)
                {
                    Runtime.World.LevelHero(Handle, level, showEffect);
                }
                else if (level < now)
                {
                    Runtime.World.StripLevels(Handle, now - level);
                }

                // The last values are what the game took, which the getters keep.
                lastLevel = Level;
                lastExperience = Experience;
            }
        }

        /// <summary>The hero's base strength, without what items and auras add. Setting it changes the base for good. Once the game has removed the hero, the last value read or set, and setting does nothing. In a declared class's constructor, the strength every hero of the class starts with (ustr); on a default object, its type's.</summary>
        [LocalSafe]
        public int Strength
        {
            get
            {
                if (Handle == null)
                {
                    return DeclaredValue<int>("ustr");
                }

                if (IsGone)
                {
                    return lastStrength;
                }

                var now = Runtime.World.StrengthOf(Handle, false);
                if (now == 0 && RemovedByGame())
                {
                    return lastStrength;
                }

                if (Local.Depth == 0)
                {
                    lastStrength = now;
                }

                return now;
            }

            set
            {
                if (Handle == null)
                {
                    Declare("ustr", value);
                    return;
                }

                if (!IsGone)
                {
                    lastStrength = value;
                    Runtime.World.SetStrength(Handle, value);
                }
            }
        }

        /// <summary>The hero's base agility, without what items and auras add. Setting it changes the base for good. Once the game has removed the hero, the last value read or set, and setting does nothing. In a declared class's constructor, the agility every hero of the class starts with (uagi); on a default object, its type's.</summary>
        [LocalSafe]
        public int Agility
        {
            get
            {
                if (Handle == null)
                {
                    return DeclaredValue<int>("uagi");
                }

                if (IsGone)
                {
                    return lastAgility;
                }

                var now = Runtime.World.AgilityOf(Handle, false);
                if (now == 0 && RemovedByGame())
                {
                    return lastAgility;
                }

                if (Local.Depth == 0)
                {
                    lastAgility = now;
                }

                return now;
            }

            set
            {
                if (Handle == null)
                {
                    Declare("uagi", value);
                    return;
                }

                if (!IsGone)
                {
                    lastAgility = value;
                    Runtime.World.SetAgility(Handle, value);
                }
            }
        }

        /// <summary>The hero's base intelligence, without what items and auras add. Setting it changes the base for good. Once the game has removed the hero, the last value read or set, and setting does nothing. In a declared class's constructor, the intelligence every hero of the class starts with (uint); on a default object, its type's.</summary>
        [LocalSafe]
        public int Intelligence
        {
            get
            {
                if (Handle == null)
                {
                    return DeclaredValue<int>("uint");
                }

                if (IsGone)
                {
                    return lastIntelligence;
                }

                var now = Runtime.World.IntelligenceOf(Handle, false);
                if (now == 0 && RemovedByGame())
                {
                    return lastIntelligence;
                }

                if (Local.Depth == 0)
                {
                    lastIntelligence = now;
                }

                return now;
            }

            set
            {
                if (Handle == null)
                {
                    Declare("uint", value);
                    return;
                }

                if (!IsGone)
                {
                    lastIntelligence = value;
                    Runtime.World.SetIntelligence(Handle, value);
                }
            }
        }

        /// <summary>The hero's strength with what items and auras add; once the game has removed the hero, the last value read.</summary>
        [LocalSafe]
        public int StrengthWithBonus
        {
            get
            {
                if (IsGone)
                {
                    return lastStrengthWithBonus;
                }

                var now = Runtime.World.StrengthOf(Handle, true);
                if (now == 0 && RemovedByGame())
                {
                    return lastStrengthWithBonus;
                }

                if (Local.Depth == 0)
                {
                    lastStrengthWithBonus = now;
                }

                return now;
            }
        }

        /// <summary>The hero's agility with what items and auras add; once the game has removed the hero, the last value read.</summary>
        [LocalSafe]
        public int AgilityWithBonus
        {
            get
            {
                if (IsGone)
                {
                    return lastAgilityWithBonus;
                }

                var now = Runtime.World.AgilityOf(Handle, true);
                if (now == 0 && RemovedByGame())
                {
                    return lastAgilityWithBonus;
                }

                if (Local.Depth == 0)
                {
                    lastAgilityWithBonus = now;
                }

                return now;
            }
        }

        /// <summary>The hero's intelligence with what items and auras add; once the game has removed the hero, the last value read.</summary>
        [LocalSafe]
        public int IntelligenceWithBonus
        {
            get
            {
                if (IsGone)
                {
                    return lastIntelligenceWithBonus;
                }

                var now = Runtime.World.IntelligenceOf(Handle, true);
                if (now == 0 && RemovedByGame())
                {
                    return lastIntelligenceWithBonus;
                }

                if (Local.Depth == 0)
                {
                    lastIntelligenceWithBonus = now;
                }

                return now;
            }
        }

        /// <summary>The skill points the hero has to spend. Setting it adds or takes the difference (UnitModifySkillPoints). Once the game has removed the hero, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public int SkillPoints
        {
            get
            {
                if (IsGone)
                {
                    return lastSkillPoints;
                }

                var now = Runtime.World.SkillPointsOf(Handle);
                if (now == 0 && RemovedByGame())
                {
                    return lastSkillPoints;
                }

                if (Local.Depth == 0)
                {
                    lastSkillPoints = now;
                }

                return now;
            }

            set
            {
                if (!IsGone)
                {
                    var delta = value - Runtime.World.SkillPointsOf(Handle);
                    if (delta != 0 && Runtime.World.AddSkillPoints(Handle, delta))
                    {
                        lastSkillPoints = value;
                    }
                }
            }
        }

        /// <summary>Revives the hero at a place (ReviveHero), with the revive effect unless showEffect is false. Gives false, and does not ask the game, when the hero is alive or gone.</summary>
        public bool Revive(float x, float y, bool showEffect = true)
        {
            if (IsGone || IsAlive)
            {
                return false;
            }

            return Runtime.World.ReviveAt(Handle, x, y, showEffect);
        }
    }
}
