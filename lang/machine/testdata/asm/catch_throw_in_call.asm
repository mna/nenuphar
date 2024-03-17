### nofail: 2

program:
	constants:
		int 1        # 0
		string "a"   # 1
		int 2        # 2

# do
# 	catch
# 		return 2
# 	end
#		return fn()
# end
function: top 3 0
	catches:
		3 6 1
	code:
		JMP  3        # goto maketuple
		CONSTANT 2    # 2
		RETURN        # no need to end with CATCHJMP as it would be unreachable

		# 3
		MAKETUPLE 0 
		MAKEFUNC 0  # fn
		CALL 0
		RETURN

# return 1 + "a"; throws
function: fn 2 0
	code:
		CONSTANT 0 # 1
		CONSTANT 1 # "a"
		PLUS			 # 1 + "a"; throws
		RETURN
