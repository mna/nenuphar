### nofail: 2
### x: 1

program:
	names:
		G
	constants:
		int 1        # 0
		string "a"   # 1
		int 2        # 2
		string "x"   # 3

# do
#   defer
#     G.x = 1
#   end
# 	catch
# 		return 2
# 	end
#		return fn()
# end
function: top 4 0 # 4 because MAKEFUNC + CALL never returns (never pops) TODO: keep in mind when computing max stack
	defers:
		6 14 1
	catches:
		10 14 7
	code:
		JMP 6
		PREDECLARED 0 # G
		CONSTANT 3    # x
		CONSTANT 0    # 1
		SETINDEX      # G.x = 1
		DEFEREXIT

		# 6
		JMP  10       # goto maketuple
		CONSTANT 2    # 2
		RUNDEFER
		RETURN        # no need to end with CATCHJMP as it would be unreachable

		# 10
		MAKETUPLE 0
		MAKEFUNC 0  # fn
		CALL 0
		RUNDEFER
		RETURN

# return 1 + "a"; throws
function: fn 2 0
	code:
		CONSTANT 0 # 1
		CONSTANT 1 # "a"
		PLUS			 # 1 + "a"; throws
		RETURN
