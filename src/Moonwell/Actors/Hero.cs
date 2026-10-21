namespace Moonwell
{
    /// <summary>A hero unit: the class of a unit the game makes as a hero when no class is registered for its type. It has experience, a level, stats and skill points, and can be revived.</summary>
    public class Hero : Unit
    {
        /// <summary>The experience last read or set.</summary>
        private int lastExperience;

        /// <summary>The base strength last read or set.</summary>
        private int lastStrength;

        /// <summary>The base agility last read or set.</summary>
        private int lastAgility;

        /// <summary>The base intelligence last read or set.</summary>
        private int lastIntelligence;

        /// <summary>The strength with bonuses last read.</summary>
        private int lastStrengthWithBonus;

        /// <summary>The agility with bonuses last read.</summary>
        private int lastAgilityWithBonus;

        /// <summary>The intelligence with bonuses last read.</summary>
        private int lastIntelligenceWithBonus;

        /// <summary>The skill points last read or set.</summary>
        private int lastSkillPoints;

        /// <summary>The hero's experience. Setting it makes the game give the levels that come with it, with the level-up effect. Once the game has removed the hero, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public int Experience
        {
            get
            {
                if (!IsGone)
                {
                    lastExperience = Runtime.World.ExperienceOf(Handle);
                }

                return lastExperience;
            }

            set
            {
                if (!IsGone)
                {
                    lastExperience = value;
                    Runtime.World.SetExperience(Handle, value, true);
                }
            }
        }

        /// <summary>Gives the hero a level (SetHeroLevel), with the level-up effect unless showEffect is false. Level reads it. Does nothing once the hero is gone.</summary>
        public void SetLevel(int level, bool showEffect = true)
        {
            if (!IsGone)
            {
                Runtime.World.LevelHero(Handle, level, showEffect);
            }
        }

        /// <summary>The hero's base strength, without what items and auras add. Setting it changes the base for good. Once the game has removed the hero, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public int Strength
        {
            get
            {
                if (!IsGone)
                {
                    lastStrength = Runtime.World.StrengthOf(Handle, false);
                }

                return lastStrength;
            }

            set
            {
                if (!IsGone)
                {
                    lastStrength = value;
                    Runtime.World.SetStrength(Handle, value);
                }
            }
        }

        /// <summary>The hero's base agility, without what items and auras add. Setting it changes the base for good. Once the game has removed the hero, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public int Agility
        {
            get
            {
                if (!IsGone)
                {
                    lastAgility = Runtime.World.AgilityOf(Handle, false);
                }

                return lastAgility;
            }

            set
            {
                if (!IsGone)
                {
                    lastAgility = value;
                    Runtime.World.SetAgility(Handle, value);
                }
            }
        }

        /// <summary>The hero's base intelligence, without what items and auras add. Setting it changes the base for good. Once the game has removed the hero, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public int Intelligence
        {
            get
            {
                if (!IsGone)
                {
                    lastIntelligence = Runtime.World.IntelligenceOf(Handle, false);
                }

                return lastIntelligence;
            }

            set
            {
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
                if (!IsGone)
                {
                    lastStrengthWithBonus = Runtime.World.StrengthOf(Handle, true);
                }

                return lastStrengthWithBonus;
            }
        }

        /// <summary>The hero's agility with what items and auras add; once the game has removed the hero, the last value read.</summary>
        [LocalSafe]
        public int AgilityWithBonus
        {
            get
            {
                if (!IsGone)
                {
                    lastAgilityWithBonus = Runtime.World.AgilityOf(Handle, true);
                }

                return lastAgilityWithBonus;
            }
        }

        /// <summary>The hero's intelligence with what items and auras add; once the game has removed the hero, the last value read.</summary>
        [LocalSafe]
        public int IntelligenceWithBonus
        {
            get
            {
                if (!IsGone)
                {
                    lastIntelligenceWithBonus = Runtime.World.IntelligenceOf(Handle, true);
                }

                return lastIntelligenceWithBonus;
            }
        }

        /// <summary>The skill points the hero has to spend. Setting it adds or takes the difference (UnitModifySkillPoints). Once the game has removed the hero, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public int SkillPoints
        {
            get
            {
                if (!IsGone)
                {
                    lastSkillPoints = Runtime.World.SkillPointsOf(Handle);
                }

                return lastSkillPoints;
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
