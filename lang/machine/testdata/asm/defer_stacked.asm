### x: 1
### y: 2
### z: 3

program:
	names:
		G
	constants:
		int 0        # 0
		int 1        # 1
		string "x"   # 2
		string "y"   # 3
		string "z"   # 4

# defer
#   i = i + 1
# 	G.z = i
# end
# defer
#   i = i + 1
# 	G.y = i
# end
# defer
#   i = i + 1
# 	G.x = i
# end
#	i = 0
function: top 3 0
	locals:
		i
	defers:
		10 34 1
		20 34 11
		30 34 21
	code:
		JMP  10      # goto next defer
		CONSTANT  1  # 1
		LOCAL 0      # i
		PLUS
		SETLOCAL 0   # i = i + 1
		PREDECLARED 0 # G
		CONSTANT 4    # z
		LOCAL 0       # i
		SETINDEX      # G.z = i
		DEFEREXIT

		# 10
		JMP  20      # goto next defer
		CONSTANT  1  # 1
		LOCAL 0      # i
		PLUS
		SETLOCAL 0   # i = i + 1
		PREDECLARED 0 # G
		CONSTANT 3    # y
		LOCAL 0       # i
		SETINDEX      # G.y = i
		DEFEREXIT

		# 20
		JMP  30      # goto main
		CONSTANT  1  # 1
		LOCAL 0      # i
		PLUS
		SETLOCAL 0   # i = i + 1
		PREDECLARED 0 # G
		CONSTANT 2    # x
		LOCAL 0       # i
		SETINDEX      # G.x = i
		DEFEREXIT

		# 30
		CONSTANT 0  # 0
		SETLOCAL 0  # i = 0
		NIL
		RUNDEFER
		RETURN

